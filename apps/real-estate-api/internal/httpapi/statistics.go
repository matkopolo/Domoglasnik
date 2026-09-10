package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
)

func (s *server) statisticsOverview(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	result, err := s.statistics.Overview(r.Context(), filter)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("overview statistics", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not calculate overview statistics")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) statisticsTimeSeries(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "year"
	}
	if interval != "month" && interval != "quarter" && interval != "year" {
		writeError(w, r, http.StatusBadRequest, "invalid_interval", "interval must be month, quarter, or year")
		return
	}
	items, err := s.statistics.TimeSeries(r.Context(), filter, interval)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("time-series statistics", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not calculate time-series statistics")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"interval": interval,
		"items":    items,
		"count":    len(items),
	})
}

func (s *server) statisticsOutliers(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	limit, ok := boundedInteger(w, r, "limit", 50, 1, 100)
	if !ok {
		return
	}
	offset, ok := boundedInteger(w, r, "offset", 0, 0, 1000000)
	if !ok {
		return
	}
	items, total, err := s.statistics.Outliers(r.Context(), filter, limit, offset)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("list statistical outliers", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load statistical outliers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "total": total,
		"limit": limit, "offset": offset,
	})
}

func (s *server) transactionExtremes(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("transactionKind")
	if kind == "" {
		kind = "sale"
	}
	if kind != "sale" && kind != "rent" {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_kind",
			"transactionKind must be sale or rent")
		return
	}
	limit, ok := boundedInteger(w, r, "limit", 3, 1, 10)
	if !ok {
		return
	}
	items, err := s.statistics.TransactionExtremes(r.Context(), kind, filter, limit)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("transaction extremes", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not calculate transaction extremes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "population": "representative",
	})
}

func statisticsFilter(w http.ResponseWriter, r *http.Request) (analytics.StatisticsFilter, bool) {
	var filter analytics.StatisticsFilter
	var ok bool
	if filter.FromDate, ok = optionalDate(w, r, "from"); !ok {
		return filter, false
	}
	if filter.ToDate, ok = optionalDate(w, r, "to"); !ok {
		return filter, false
	}
	if filter.FromDate != nil && filter.ToDate != nil && filter.FromDate.After(*filter.ToDate) {
		writeError(w, r, http.StatusBadRequest, "invalid_date_range", "from must not be after to")
		return filter, false
	}
	if filter.MinArea, ok = optionalPositiveFloat(w, r, "minArea"); !ok {
		return filter, false
	}
	if filter.MaxArea, ok = optionalPositiveFloat(w, r, "maxArea"); !ok {
		return filter, false
	}
	if filter.MinArea != nil && filter.MaxArea != nil && *filter.MinArea > *filter.MaxArea {
		writeError(w, r, http.StatusBadRequest, "invalid_area_range", "minArea must not exceed maxArea")
		return filter, false
	}
	filter.PropertyGroup = r.URL.Query().Get("propertyGroup")
	if filter.PropertyGroup != "" && !validPropertyGroup(filter.PropertyGroup) {
		writeError(w, r, http.StatusBadRequest, "invalid_property_group", "unsupported propertyGroup")
		return filter, false
	}
	filter.LandScope = r.URL.Query().Get("landScope")
	if filter.LandScope == "" {
		filter.LandScope = "strict"
	}
	switch filter.LandScope {
	case "strict", "broad", "all", "agricultural", "forest", "unverified", "mixed", "other":
	default:
		writeError(w, r, http.StatusBadRequest, "invalid_land_scope",
			"landScope must be strict, broad, all, agricultural, forest, unverified, mixed, or other")
		return filter, false
	}
	if value := r.URL.Query().Get("geographyId"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			writeError(w, r, http.StatusBadRequest, "invalid_geography_id", "geographyId must be a positive integer")
			return filter, false
		}
		filter.GeographyID = &parsed
	}
	return filter, true
}

func optionalPositiveFloat(w http.ResponseWriter, r *http.Request, name string) (*float64, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 || parsed > 1000000 {
		writeError(w, r, http.StatusBadRequest, "invalid_area", name+" must be a positive number")
		return nil, false
	}
	return &parsed, true
}

func optionalDate(w http.ResponseWriter, r *http.Request, name string) (*time.Time, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_date", name+" must use YYYY-MM-DD")
		return nil, false
	}
	return &parsed, true
}

func validPropertyGroup(group string) bool {
	switch group {
	case "apartment", "house", "land", "commercial", "garage", "room", "mixed", "unknown":
		return true
	default:
		return false
	}
}

func boundedInteger(
	w http.ResponseWriter,
	r *http.Request,
	name string,
	fallback, minimum, maximum int,
) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		writeError(w, r, http.StatusBadRequest, "invalid_"+name,
			name+" is outside the supported range")
		return 0, false
	}
	return parsed, true
}
