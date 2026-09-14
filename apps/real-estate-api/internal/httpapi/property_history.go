package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/economics"
)

func (s *server) propertyHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionID(w, r)
	if !ok {
		return
	}
	if s.histories == nil {
		writeError(w, r, 503, "history_unavailable", "property history unavailable")
		return
	}
	result, err := s.histories.PropertyHistory(r.Context(), id)
	if transactionError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) resaleSummary(w http.ResponseWriter, r *http.Request) {
	filter, ok := statisticsFilter(w, r)
	if !ok {
		return
	}
	if s.histories == nil {
		writeError(w, r, 503, "history_unavailable", "property history unavailable")
		return
	}
	result, err := s.histories.ResaleSummary(r.Context(), filter)
	if errors.Is(err, analytics.ErrGeographyNotFound) {
		writeError(w, r, 404, "geography_not_found", "geography not found")
		return
	}
	if err != nil {
		s.logger.Error("resale summary", "error", err)
		writeError(w, r, 500, "query_failed", "resale summary unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) inflationFactor(w http.ResponseWriter, r *http.Request) {
	from, ok := optionalDate(w, r, "from")
	if !ok {
		return
	}
	to, ok := optionalDate(w, r, "to")
	if !ok {
		return
	}
	if from == nil || to == nil || from.After(*to) || from.Year() < 1991 || to.After(time.Now().UTC()) {
		writeError(w, r, 400, "invalid_date_range", "from/to must be ordered dates from 1991 through today")
		return
	}
	store, ok := s.economics.(interface {
		InflationFactor(context.Context, time.Time, time.Time) (economics.InflationFactor, error)
	})
	if !ok {
		writeError(w, r, 503, "inflation_unavailable", "inflation adjustment unavailable")
		return
	}
	result, err := store.InflationFactor(r.Context(), *from, *to)
	if err != nil {
		s.logger.Warn("inflation factor", "error", err)
		writeError(w, r, 503, "inflation_unavailable", "complete monthly inflation series unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
