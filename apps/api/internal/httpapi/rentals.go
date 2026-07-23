package httpapi

import (
	"errors"
	"net/http"

	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
)

func (s *server) rentalOverview(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	result, err := s.rentals.RentalOverview(r.Context(), filter)
	if rentalError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) rentalTimeSeries(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "year"
	}
	if interval != "month" && interval != "quarter" && interval != "year" {
		writeError(w, r, http.StatusBadRequest, "invalid_interval",
			"interval must be month, quarter, or year")
		return
	}
	items, err := s.rentals.RentalTimeSeries(r.Context(), filter, interval)
	if rentalError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"interval": interval, "items": items, "count": len(items),
	})
}

func (s *server) rentalOutliers(w http.ResponseWriter, r *http.Request) {
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
	items, total, err := s.rentals.RentalOutliers(
		r.Context(), filter, limit, offset)
	if rentalError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "total": total,
		"limit": limit, "offset": offset,
	})
}

func rentalError(w http.ResponseWriter, r *http.Request, s *server, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found",
			"geography was not found")
		return true
	}
	s.logger.Error("rental analytics", "error", err)
	writeError(w, r, http.StatusInternalServerError, "query_failed",
		"could not calculate rental analytics")
	return true
}

func (s *server) rentalMapClusters(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.rentals.RentalClusters(
		r.Context(), filter, bounds, clusterGridSize(zoom))
	if rentalError(w, r, s, err) {
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
				"transactionCount":        item.TransactionCount,
				"representativeCount":     item.RepresentativeCount,
				"medianMonthlyRentPerSqm": item.MedianPricePerSQM,
			},
		})
	}
	writeGeoJSON(w, http.StatusOK, map[string]any{
		"type": "FeatureCollection", "features": features,
		"count": len(features), "zoom": zoom,
	})
}

func (s *server) rentalMapProperties(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.rentals.RentalPropertyPoints(r.Context(), filter, bounds, limit)
	if rentalError(w, r, s, err) {
		return
	}
	features := make([]map[string]any, 0, len(items))
	for _, item := range items {
		features = append(features, map[string]any{
			"type": "Feature", "id": item.TransactionID,
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
