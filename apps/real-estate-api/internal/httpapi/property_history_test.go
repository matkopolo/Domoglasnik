package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
)

type fakeHistory struct{ err error }

func (f fakeHistory) PropertyHistory(context.Context, int64) (analytics.PropertyHistory, error) {
	return analytics.PropertyHistory{Assets: []analytics.HistoryAsset{}}, f.err
}
func (f fakeHistory) ResaleSummary(context.Context, analytics.StatisticsFilter) (analytics.ResaleSummary, error) {
	return analytics.ResaleSummary{}, f.err
}

func TestHistoryHTTP(t *testing.T) {
	for _, test := range []struct {
		path   string
		err    error
		status int
	}{
		{"/api/v1/sales/transactions/1/history", nil, 200},
		{"/api/v1/sales/transactions/0/history", nil, 400},
		{"/api/v1/sales/transactions/1/history", analytics.ErrTransactionNotFound, 404},
		{"/api/v1/sales/transactions/1/history", errors.New("db unavailable"), 500},
		{"/api/v1/sales/statistics/resales?minArea=100&maxArea=50", nil, 400},
		{"/api/v1/sales/statistics/resales?landScope=agricultural", nil, 200},
		{"/api/v1/sales/statistics/resales?geographyId=1", analytics.ErrGeographyNotFound, 404},
		{"/api/v1/economy/inflation-factor?from=2025-01-01", nil, 400},
		{"/api/v1/economy/inflation-factor?from=2025-03-01&to=2025-01-01", nil, 400},
		{"/api/v1/economy/inflation-factor?from=2025-01-01&to=2025-03-01", nil, 503},
	} {
		t.Run(test.path, func(t *testing.T) {
			handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Histories: fakeHistory{test.err}})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
