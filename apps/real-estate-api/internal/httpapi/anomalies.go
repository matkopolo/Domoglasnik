package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
)

func (s *server) anomalies(w http.ResponseWriter, r *http.Request) {
	filter, ok := anomalyFilter(w, r)
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
	items, total, err := s.anomaliesStore.Anomalies(
		r.Context(), filter, limit, offset)
	if err != nil {
		s.logger.Error("list anomalies", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not load anomaly queue")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "total": total,
		"limit": limit, "offset": offset,
	})
}

type anomalyReviewRequest struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

func (s *server) reviewAnomaly(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.authenticatedUser(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	if !validTransactionKind(kind) || kind == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_kind",
			"transaction kind must be sale or rent")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_id",
			"transaction id must be a non-zero integer")
		return
	}
	var request anomalyReviewRequest
	if !decodeStrictJSON(w, r, maxAuthBody, &request) {
		return
	}
	if !validReviewStatus(request.Status) || request.Status == "unreviewed" {
		writeError(w, r, http.StatusBadRequest, "invalid_review_status",
			"review status must contain a completed review decision")
		return
	}
	if request.Notes != nil {
		trimmed := strings.TrimSpace(*request.Notes)
		if len(trimmed) > 2000 {
			writeError(w, r, http.StatusBadRequest, "invalid_review_notes",
				"review notes cannot exceed 2000 characters")
			return
		}
		request.Notes = &trimmed
	}
	err = s.anomaliesStore.ReviewAnomaly(
		r.Context(), kind, id, request.Status, request.Notes, user.ID)
	if errors.Is(err, analytics.ErrAnomalyNotFound) {
		writeError(w, r, http.StatusNotFound, "anomaly_not_found",
			"anomaly signal was not found")
		return
	}
	if err != nil {
		s.logger.Error("review anomaly", "error", err)
		writeError(w, r, http.StatusInternalServerError, "review_failed",
			"could not save anomaly review")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) anomalySummary(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("transactionKind")
	if !validTransactionKind(kind) {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_kind",
			"transactionKind must be sale or rent")
		return
	}
	result, err := s.anomaliesStore.AnomalySummary(r.Context(), kind)
	if err != nil {
		s.logger.Error("anomaly summary", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not summarize anomalies")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func anomalyFilter(w http.ResponseWriter, r *http.Request) (analytics.AnomalyFilter, bool) {
	filter := analytics.AnomalyFilter{
		TransactionKind: r.URL.Query().Get("transactionKind"),
		Flag:            r.URL.Query().Get("flag"),
		ReviewStatus:    r.URL.Query().Get("reviewStatus"),
	}
	if !validTransactionKind(filter.TransactionKind) {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_kind",
			"transactionKind must be sale or rent")
		return filter, false
	}
	if filter.ReviewStatus != "" && !validReviewStatus(filter.ReviewStatus) {
		writeError(w, r, http.StatusBadRequest, "invalid_review_status",
			"unsupported reviewStatus")
		return filter, false
	}
	if value := r.URL.Query().Get("minimumScore"); value != "" {
		score, err := strconv.ParseFloat(value, 64)
		if err != nil || score < 0 || score > 1 {
			writeError(w, r, http.StatusBadRequest, "invalid_minimum_score",
				"minimumScore must be between 0 and 1")
			return filter, false
		}
		filter.MinimumScore = score
	}
	return filter, true
}

func validTransactionKind(kind string) bool {
	return kind == "" || kind == "sale" || kind == "rent"
}

func validReviewStatus(status string) bool {
	switch status {
	case "unreviewed", "confirmed_valid", "confirmed_error", "needs_follow_up", "ignored":
		return true
	default:
		return false
	}
}
