package httpapi

import (
	"errors"
	"net/http"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
)

func (s *server) saleDistribution(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "pricePerSqm"
	}
	if metric != "price" && metric != "pricePerSqm" {
		writeError(w, r, http.StatusBadRequest, "invalid_metric",
			"metric must be price or pricePerSqm")
		return
	}
	bins, ok := boundedInteger(w, r, "bins", 20, 5, 100)
	if !ok {
		return
	}
	result, err := s.distributions.SaleDistribution(r.Context(), filter, metric, bins)
	if distributionError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) rentalDistribution(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "rentPerSqm"
	}
	if metric != "monthlyRent" && metric != "rentPerSqm" {
		writeError(w, r, http.StatusBadRequest, "invalid_metric",
			"metric must be monthlyRent or rentPerSqm")
		return
	}
	bins, ok := boundedInteger(w, r, "bins", 20, 5, 100)
	if !ok {
		return
	}
	result, err := s.distributions.RentalDistribution(r.Context(), filter, metric, bins)
	if distributionError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func distributionError(w http.ResponseWriter, r *http.Request, s *server, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, http.StatusNotFound, "geography_not_found",
			"geography was not found")
		return true
	}
	s.logger.Error("distribution", "error", err)
	writeError(w, r, http.StatusInternalServerError, "query_failed",
		"could not calculate distribution")
	return true
}
