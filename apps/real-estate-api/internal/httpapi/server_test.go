package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/geography"
)

func TestLive(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}
}

func TestReadyFailsWhenDatabaseFails(t *testing.T) {
	response := httptest.NewRecorder()
	handler := newTestHandler(errors.New("database unavailable"), fakeGeographies{})
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestMeta(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestDataStatus(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/api/v1/data-status", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"current"`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestGeographiesValidateLevel(t *testing.T) {
	response := httptest.NewRecorder()
	handler := newTestHandler(nil, fakeGeographies{})
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/geographies?level=neighborhood", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestGeographyChildrenNotFound(t *testing.T) {
	response := httptest.NewRecorder()
	handler := newTestHandler(nil, fakeGeographies{err: geography.ErrNotFound})
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/geographies/99/children", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestUnknownRouteUsesAPIError(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/does-not-exist", nil),
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
}

func TestGetGeography(t *testing.T) {
	response := httptest.NewRecorder()
	store := fakeGeographies{units: []geography.Unit{{ID: 1, Level: "country", Name: "Slovenija", GURSEID: "SI"}}}
	newTestHandler(nil, store).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/geographies/1", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestBoundaryRejectsInvalidDetail(t *testing.T) {
	response := httptest.NewRecorder()
	store := fakeGeographies{units: []geography.Unit{{ID: 1}}}
	newTestHandler(nil, store).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/geographies/1/boundary?detail=huge", nil),
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestBoundaryReturnsGeoJSONFeature(t *testing.T) {
	response := httptest.NewRecorder()
	store := fakeGeographies{units: []geography.Unit{{ID: 1, Level: "country", Name: "Slovenija", GURSEID: "SI"}}}
	newTestHandler(nil, store).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/geographies/1/boundary?detail=low", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if body := response.Body.String(); !strings.Contains(body, `"type":"Feature"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestStatisticsOverview(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet,
			"/api/v1/statistics/overview?from=2024-01-01&to=2024-12-31&propertyGroup=apartment",
			nil),
	)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"transactionCount":10`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStatisticsRejectInvalidFilters(t *testing.T) {
	tests := []string{
		"/api/v1/statistics/overview?from=2024-13-01",
		"/api/v1/statistics/overview?from=2025-01-01&to=2024-01-01",
		"/api/v1/statistics/overview?propertyGroup=castle",
		"/api/v1/statistics/overview?propertyGroup=land&landScope=speculative",
		"/api/v1/statistics/overview?geographyId=-1",
		"/api/v1/statistics/overview?minArea=0",
		"/api/v1/statistics/overview?maxArea=nope",
		"/api/v1/statistics/overview?minArea=90&maxArea=60",
		"/api/v1/statistics/timeseries?interval=week",
	}
	for _, path := range tests {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil),
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestStatisticsFilterDefaultsFrontendLandRequestsToStrict(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet, "/api/v1/statistics/overview?propertyGroup=land", nil)
	filter, ok := statisticsFilter(response, request)
	if !ok || filter.LandScope != "strict" {
		t.Fatalf("ok=%v landScope=%q body=%s",
			ok, filter.LandScope, response.Body.String())
	}
}

func TestStatisticsFilterAcceptsSeparateLandCategories(t *testing.T) {
	for _, scope := range []string{"strict", "broad", "all", "agricultural", "forest", "unverified", "mixed", "other"} {
		response := httptest.NewRecorder()
		filter, ok := statisticsFilter(response, httptest.NewRequest(http.MethodGet, "/api/v1/statistics/overview?propertyGroup=land&landScope="+scope, nil))
		if !ok || filter.LandScope != scope {
			t.Fatalf("scope=%s parsed=%s ok=%v", scope, filter.LandScope, ok)
		}
	}
}

func TestStatisticsTimeSeries(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/statistics/timeseries?interval=quarter", nil),
	)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"interval":"quarter"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStatisticsOutliers(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/statistics/outliers?limit=10&offset=0", nil),
	)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"total":1`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTransactionExtremes(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet,
			"/api/v1/transactions/extremes?transactionKind=sale&propertyGroup=apartment&limit=1", nil),
	)
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"population":"representative"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRentalStatisticsEndpoints(t *testing.T) {
	for _, path := range []string{
		"/api/v1/rentals/statistics/overview?propertyGroup=room",
		"/api/v1/rentals/statistics/timeseries?interval=month",
		"/api/v1/rentals/statistics/outliers?limit=10",
		"/api/v1/rentals/map/clusters?bbox=15,46,16,47",
		"/api/v1/rentals/map/properties?bbox=15,46,16,47",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestInflationEndpoint(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(response, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/economy/inflation?from=2024-01-01&to=2024-12-31&interval=year", nil,
	))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"annualRatePercent":3.2`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTransactionDetailEndpoints(t *testing.T) {
	for _, path := range []string{
		"/api/v1/sales/transactions/1",
		"/api/v1/rentals/transactions/1",
		"/api/v1/sales/transactions/1/context",
		"/api/v1/rentals/transactions/1/context",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil),
		)
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"transactionId":1`) {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestTransactionDetailRejectsInvalidID(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet,
			"/api/v1/sales/transactions/nope", nil),
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTransactionDetailAcceptsNegativeSourceID(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet,
			"/api/v1/sales/transactions/-123", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTransactionDetailNotFound(t *testing.T) {
	handler := New(Dependencies{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Database: fakeDatabase{}, Geographies: fakeGeographies{},
		Statistics: fakeStatistics{}, Rentals: fakeStatistics{}, Maps: fakeMaps{},
		Transactions:  fakeTransactions{err: analytics.ErrTransactionNotFound},
		Distributions: fakeDistributions{},
		Buildings:     fakeBuildings{},
		Anomalies:     fakeAnomalies{},
		Identities:    fakeIdentities{},
		Models:        fakeModels{},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/rentals/transactions/999", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestIndexAndValuationEndpoints(t *testing.T) {
	for _, path := range []string{
		"/api/v1/indices?transactionKind=sale&method=hedonic&propertyGroup=apartment",
		"/api/v1/valuations/sale/-1",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestIdentityAndPreferenceEndpoints(t *testing.T) {
	tests := []struct {
		method, path, body string
		auth               bool
		status             int
	}{
		{http.MethodPost, "/api/v1/auth/register",
			`{"email":"user@example.com","password":"very-secure-password","displayName":"User"}`,
			false, http.StatusCreated},
		{http.MethodPost, "/api/v1/auth/login",
			`{"email":"user@example.com","password":"very-secure-password"}`,
			false, http.StatusOK},
		{http.MethodGet, "/api/v1/users/me", "", true, http.StatusOK},
		{http.MethodGet, "/api/v1/users/me/preferences", "", true, http.StatusOK},
		{http.MethodPut, "/api/v1/users/me/preferences", `{"theme":"light"}`,
			true, http.StatusOK},
		{http.MethodPost, "/api/v1/auth/logout", "", true, http.StatusNoContent},
	}
	handler := newTestHandler(nil, fakeGeographies{})
	for _, test := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.auth {
			request.Header.Set("Authorization", "Bearer token")
		}
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%s %s status=%d body=%s",
				test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestIdentityValidationAndAuthentication(t *testing.T) {
	tests := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/auth/register",
			`{"email":"bad","password":"short","displayName":""}`},
		{http.MethodGet, "/api/v1/users/me", ""},
		{http.MethodPut, "/api/v1/users/me/preferences", `[]`},
	}
	handler := newTestHandler(nil, fakeGeographies{})
	for _, test := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(
			test.method, test.path, strings.NewReader(test.body)))
		if response.Code < 400 {
			t.Fatalf("%s %s status=%d", test.method, test.path, response.Code)
		}
	}
}

func TestAnomalyEndpoints(t *testing.T) {
	for _, path := range []string{
		"/api/v1/anomalies?transactionKind=sale&minimumScore=0.2",
		"/api/v1/anomalies/summary?transactionKind=rent",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestAnomalyEndpointsRejectInvalidFilters(t *testing.T) {
	for _, path := range []string{
		"/api/v1/anomalies?transactionKind=transfer",
		"/api/v1/anomalies?minimumScore=2",
		"/api/v1/anomalies?reviewStatus=deleted",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d", path, response.Code)
		}
	}
}

func TestReviewAnomalyRequiresAuthAndAcceptsSignedID(t *testing.T) {
	handler := newTestHandler(nil, fakeGeographies{})
	body := `{"status":"confirmed_valid","notes":"checked source"}`
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPatch,
		"/api/v1/anomalies/sale/-123/review", strings.NewReader(body)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch,
		"/api/v1/anomalies/sale/-123/review", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBuildingStatistics(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet,
			"/api/v1/buildings/657/123/statistics", nil))
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"buildingNumber":"123"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBuildingStatisticsRejectsInvalidIdentity(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(nil, fakeGeographies{}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet,
			"/api/v1/buildings/nope/123/statistics", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBuildingStatisticsRejectsInvalidFilter(t *testing.T) {
	for _, path := range []string{
		"/api/v1/buildings/657/123/statistics?from=2024-01-01&to=2023-01-01",
		"/api/v1/buildings/657/123/statistics?propertyGroup=castle",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s",
				path, response.Code, response.Body.String())
		}
	}
}

func TestDistributionEndpoints(t *testing.T) {
	for _, path := range []string{
		"/api/v1/statistics/distribution?metric=price&bins=10",
		"/api/v1/rentals/statistics/distribution?metric=rentPerSqm&bins=10",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"sampleSize":10`) {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestDistributionRejectsInvalidOptions(t *testing.T) {
	for _, path := range []string{
		"/api/v1/statistics/distribution?metric=rent",
		"/api/v1/rentals/statistics/distribution?bins=101",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d", path, response.Code)
		}
	}
}

func TestMapEndpointsRequireValidBounds(t *testing.T) {
	tests := []string{
		"/api/v1/map/clusters",
		"/api/v1/map/clusters?bbox=16,46,15,47",
		"/api/v1/map/properties?bbox=15,46,16,47&limit=501",
	}
	for _, path := range tests {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil),
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestMapEndpointsReturnGeoJSON(t *testing.T) {
	for _, path := range []string{
		"/api/v1/map/clusters?bbox=15,46,16,47&zoom=9",
		"/api/v1/map/properties?bbox=15,46,16,47&limit=10",
	} {
		response := httptest.NewRecorder()
		newTestHandler(nil, fakeGeographies{}).ServeHTTP(
			response, httptest.NewRequest(http.MethodGet, path, nil),
		)
		if response.Code != http.StatusOK ||
			response.Header().Get("Content-Type") != "application/geo+json; charset=utf-8" ||
			!strings.Contains(response.Body.String(), `"type":"FeatureCollection"`) {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestClusterGridSize(t *testing.T) {
	if clusterGridSize(5) != 0.25 || clusterGridSize(14) != 0.003 {
		t.Fatal("unexpected map grid sizing")
	}
}
