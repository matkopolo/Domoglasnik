package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
)

func (s *server) saleTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionID(w, r)
	if !ok {
		return
	}
	result, err := s.transactions.SaleTransaction(r.Context(), id)
	if transactionError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) rentalTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := transactionID(w, r)
	if !ok {
		return
	}
	result, err := s.transactions.RentalTransaction(r.Context(), id)
	if transactionError(w, r, s, err) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func transactionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_transaction_id",
			"transaction id must be a non-zero integer")
		return 0, false
	}
	return id, true
}

func transactionError(w http.ResponseWriter, r *http.Request, s *server, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, analytics.ErrTransactionNotFound) {
		writeError(w, r, http.StatusNotFound, "transaction_not_found",
			"transaction was not found")
		return true
	}
	s.logger.Error("transaction detail", "error", err)
	writeError(w, r, http.StatusInternalServerError, "query_failed",
		"could not load transaction")
	return true
}
