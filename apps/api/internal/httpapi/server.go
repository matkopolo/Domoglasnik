package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/geography"
	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/identity"
)

type databasePinger interface {
	Ping(context.Context) error
}

type Dependencies struct {
	Logger           *slog.Logger
	Database         databasePinger
	Geographies      geography.Store
	Statistics       analytics.StatisticsStore
	Rentals          analytics.RentalStatisticsStore
	Transactions     analytics.TransactionDetailStore
	Distributions    analytics.DistributionStore
	Buildings        analytics.BuildingStore
	Anomalies        analytics.AnomalyStore
	Identities       identity.Store
	Models           analytics.ModelStore
	Maps             analytics.MapStore
	AllowedOrigins   []string
	Environment      string
	ApplicationName  string
	ApplicationBuild string
}

type server struct {
	logger           *slog.Logger
	database         databasePinger
	geographies      geography.Store
	statistics       analytics.StatisticsStore
	rentals          analytics.RentalStatisticsStore
	transactions     analytics.TransactionDetailStore
	distributions    analytics.DistributionStore
	buildings        analytics.BuildingStore
	anomaliesStore   analytics.AnomalyStore
	identities       identity.Store
	models           analytics.ModelStore
	maps             analytics.MapStore
	environment      string
	applicationName  string
	applicationBuild string
}

func New(dependencies Dependencies) http.Handler {
	s := &server{
		logger:           dependencies.Logger,
		database:         dependencies.Database,
		geographies:      dependencies.Geographies,
		statistics:       dependencies.Statistics,
		rentals:          dependencies.Rentals,
		transactions:     dependencies.Transactions,
		distributions:    dependencies.Distributions,
		buildings:        dependencies.Buildings,
		anomaliesStore:   dependencies.Anomalies,
		identities:       dependencies.Identities,
		models:           dependencies.Models,
		maps:             dependencies.Maps,
		environment:      dependencies.Environment,
		applicationName:  dependencies.ApplicationName,
		applicationBuild: dependencies.ApplicationBuild,
	}

	router := s.routes()
	return middleware(dependencies.Logger, dependencies.AllowedOrigins, router)
}

func (s *server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.database.Ping(ctx); err != nil {
		s.logger.Warn("readiness check failed", "error", err)
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "ok",
		"application": s.applicationName,
		"build":       s.applicationBuild,
		"environment": s.environment,
	})
}

func (s *server) meta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion":       "v1",
		"transactionKinds": []string{"sale", "rent"},
		"intervals":        []string{"month", "quarter", "year"},
		"geographyLevels":  []string{"country", "statistical_region", "municipality", "city_district", "local_community", "settlement"},
		"propertyGroups":   []string{"apartment", "house", "land", "commercial", "garage", "room", "mixed", "unknown"},
	})
}

func (s *server) listGeographies(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	if level != "" && !validGeographyLevel(level) {
		writeError(w, r, http.StatusBadRequest, "invalid_level", "unsupported geography level")
		return
	}

	var parentID *int64
	if value := r.URL.Query().Get("parentId"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			writeError(w, r, http.StatusBadRequest, "invalid_parent_id", "parentId must be a positive integer")
			return
		}
		parentID = &parsed
	}

	units, err := s.geographies.List(r.Context(), geography.Filter{Level: level, ParentID: parentID})
	if err != nil {
		s.logger.Error("list geographies", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load geographies")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": units, "count": len(units)})
}

func (s *server) geographyChildren(w http.ResponseWriter, r *http.Request) {
	id, ok := geographyID(w, r)
	if !ok {
		return
	}
	units, err := s.geographies.Children(r.Context(), id)
	if errors.Is(err, geography.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("list geography children", "error", err, "geography_id", id)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load geography children")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": units, "count": len(units)})
}

func (s *server) getGeography(w http.ResponseWriter, r *http.Request) {
	id, ok := geographyID(w, r)
	if !ok {
		return
	}
	unit, err := s.geographies.Get(r.Context(), id)
	if errors.Is(err, geography.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("get geography", "error", err, "geography_id", id)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load geography")
		return
	}
	writeJSON(w, http.StatusOK, unit)
}

func (s *server) geographyBoundary(w http.ResponseWriter, r *http.Request) {
	id, ok := geographyID(w, r)
	if !ok {
		return
	}
	tolerance, valid := boundaryTolerance(r.URL.Query().Get("detail"))
	if !valid {
		writeError(w, r, http.StatusBadRequest, "invalid_detail", "detail must be low, medium, or full")
		return
	}
	boundary, err := s.geographies.Boundary(r.Context(), id, tolerance)
	if errors.Is(err, geography.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("get geography boundary", "error", err, "geography_id", id)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load geography boundary")
		return
	}
	writeGeoJSON(w, http.StatusOK, map[string]any{
		"type":       "Feature",
		"id":         boundary.Unit.ID,
		"properties": boundary.Unit,
		"geometry":   boundary.Geometry,
	})
}

func geographyID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, r, http.StatusBadRequest, "invalid_geography_id", "geography id must be a positive integer")
		return 0, false
	}
	return id, true
}

func boundaryTolerance(detail string) (float64, bool) {
	switch detail {
	case "", "medium":
		return 0.0001, true
	case "low":
		return 0.001, true
	case "full":
		return 0, true
	default:
		return 0, false
	}
}

func validGeographyLevel(level string) bool {
	switch level {
	case "country", "statistical_region", "municipality", "city_district", "local_community", "settlement":
		return true
	default:
		return false
	}
}

var _ databasePinger = (*pgxpool.Pool)(nil)
