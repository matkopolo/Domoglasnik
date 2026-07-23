package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
)

func (s *server) marketIndices(w http.ResponseWriter, r *http.Request) {
	filter := analytics.MarketIndexFilter{
		TransactionKind: r.URL.Query().Get("transactionKind"),
		Method:          r.URL.Query().Get("method"),
		PropertyGroup:   r.URL.Query().Get("propertyGroup"),
	}
	if !validTransactionKind(filter.TransactionKind) {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_kind",
			"transactionKind must be sale or rent")
		return
	}
	if filter.Method != "" && filter.Method != "repeat_sales" &&
		filter.Method != "hedonic" {
		writeError(w, r, http.StatusBadRequest, "invalid_method",
			"method must be repeat_sales or hedonic")
		return
	}
	if filter.PropertyGroup != "" && !validPropertyGroup(filter.PropertyGroup) {
		writeError(w, r, http.StatusBadRequest, "invalid_property_group",
			"unsupported propertyGroup")
		return
	}
	items, err := s.models.MarketIndices(r.Context(), filter)
	if err != nil {
		s.logger.Error("market indices", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not load market indices")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "scope": "Slovenia",
	})
}

func (s *server) valuationPrediction(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "sale" && kind != "rent" {
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
	result, err := s.models.ValuationPrediction(r.Context(), kind, id)
	if errors.Is(err, analytics.ErrPredictionNotFound) {
		writeError(w, r, http.StatusNotFound, "prediction_not_found",
			"no eligible model prediction exists for this transaction")
		return
	}
	if err != nil {
		s.logger.Error("valuation prediction", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not load valuation prediction")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
