package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
)

func (s *server) mapClusters(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	bounds, ok := mapBounds(w, r)
	if !ok {
		return
	}
	zoom, ok := boundedInteger(w, r, "zoom", 8, 0, 22)
	if !ok {
		return
	}
	items, err := s.maps.Clusters(r.Context(), filter, bounds, clusterGridSize(zoom))
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("map clusters", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load map clusters")
		return
	}
	features := make([]map[string]any, 0, len(items))
	for _, item := range items {
		features = append(features, map[string]any{
			"type": "Feature",
			"geometry": map[string]any{
				"type": "Point", "coordinates": []float64{item.Longitude, item.Latitude},
			},
			"properties": map[string]any{
				"transactionCount":    item.TransactionCount,
				"representativeCount": item.RepresentativeCount,
				"medianPricePerSqm":   item.MedianPricePerSQM,
			},
		})
	}
	writeGeoJSON(w, http.StatusOK, map[string]any{
		"type": "FeatureCollection", "features": features,
		"count": len(features), "zoom": zoom,
	})
}

func (s *server) mapProperties(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	bounds, ok := mapBounds(w, r)
	if !ok {
		return
	}
	limit, ok := boundedInteger(w, r, "limit", 250, 1, 500)
	if !ok {
		return
	}
	items, err := s.maps.PropertyPoints(r.Context(), filter, bounds, limit)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("map properties", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load map properties")
		return
	}
	features := make([]map[string]any, 0, len(items))
	for _, item := range items {
		features = append(features, map[string]any{
			"type": "Feature",
			"id":   item.TransactionID,
			"geometry": map[string]any{
				"type": "Point", "coordinates": []float64{item.Longitude, item.Latitude},
			},
			"properties": item,
		})
	}
	writeGeoJSON(w, http.StatusOK, map[string]any{
		"type": "FeatureCollection", "features": features,
		"count": len(features), "limit": limit,
	})
}

func (s *server) mapGeographies(w http.ResponseWriter, r *http.Request) {
	s.geographyMap(w, r, "sale")
}

func (s *server) rentalMapGeographies(w http.ResponseWriter, r *http.Request) {
	s.geographyMap(w, r, "rent")
}

func (s *server) geographyMap(w http.ResponseWriter, r *http.Request, kind string) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	bounds, ok := mapBounds(w, r)
	if !ok {
		return
	}
	level := r.URL.Query().Get("level")
	if level == "" || !validGeographyLevel(level) {
		writeError(w, r, http.StatusBadRequest, "invalid_level", "unsupported geography level")
		return
	}
	items, err := s.maps.GeographySummaries(r.Context(), filter, bounds, level, kind)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found", "geography was not found")
		return
	}
	if err != nil {
		s.logger.Error("map geography summaries", "error", err, "level", level, "kind", kind)
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load map geography summaries")
		return
	}
	features := make([]map[string]any, 0, len(items))
	for _, item := range items {
		properties := map[string]any{
			"geographyId": item.GeographyID, "level": item.Level, "name": item.Name,
			"bounds": item.Bounds, "transactionCount": item.TransactionCount,
			"representativeCount": item.RepresentativeCount,
		}
		if kind == "sale" {
			properties["medianPricePerSqm"] = item.MedianAmountPerSQM
			properties["medianPrice"] = item.MedianAmount
		} else {
			properties["medianMonthlyRentPerSqm"] = item.MedianAmountPerSQM
			properties["medianMonthlyRent"] = item.MedianAmount
		}
		features = append(features, map[string]any{
			"type": "Feature", "id": item.GeographyID,
			"geometry": map[string]any{
				"type": "Point", "coordinates": []float64{item.Longitude, item.Latitude},
			},
			"properties": properties,
		})
	}
	writeGeoJSON(w, http.StatusOK, map[string]any{
		"type": "FeatureCollection", "features": features,
		"count": len(features), "level": level,
	})
}

func mapBounds(w http.ResponseWriter, r *http.Request) (analytics.Bounds, bool) {
	parts := strings.Split(r.URL.Query().Get("bbox"), ",")
	if len(parts) != 4 {
		writeError(w, r, http.StatusBadRequest, "invalid_bbox",
			"bbox must contain west,south,east,north")
		return analytics.Bounds{}, false
	}
	values := make([]float64, 4)
	for index, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_bbox",
				"bbox coordinates must be numbers")
			return analytics.Bounds{}, false
		}
		values[index] = value
	}
	bounds := analytics.Bounds{
		West: values[0], South: values[1], East: values[2], North: values[3],
	}
	if bounds.West < -180 || bounds.East > 180 ||
		bounds.South < -90 || bounds.North > 90 ||
		bounds.West >= bounds.East || bounds.South >= bounds.North {
		writeError(w, r, http.StatusBadRequest, "invalid_bbox",
			"bbox coordinates or ordering are invalid")
		return analytics.Bounds{}, false
	}
	return bounds, true
}

func clusterGridSize(zoom int) float64 {
	switch {
	case zoom <= 6:
		return 0.25
	case zoom <= 8:
		return 0.10
	case zoom <= 10:
		return 0.03
	case zoom <= 12:
		return 0.01
	default:
		return 0.003
	}
}
