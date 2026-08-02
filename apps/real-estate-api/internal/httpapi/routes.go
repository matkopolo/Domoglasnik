package httpapi

import "net/http"

// routes is the single source of truth for HTTP method/path registration.
// The OpenAPI contract test compares these registrations with openapi.yaml.
func (s *server) routes() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("GET /health/live", s.live)
	router.HandleFunc("GET /health/ready", s.ready)
	router.HandleFunc("GET /api/v1/health", s.ready)
	router.HandleFunc("GET /api/v1/meta", s.meta)
	router.HandleFunc("GET /api/v1/economy/inflation", s.inflation)
	router.HandleFunc("GET /api/v1/geographies", s.listGeographies)
	router.HandleFunc("GET /api/v1/geographies/{id}", s.getGeography)
	router.HandleFunc("GET /api/v1/geographies/{id}/boundary", s.geographyBoundary)
	router.HandleFunc("GET /api/v1/geographies/{id}/children", s.geographyChildren)
	router.HandleFunc("GET /api/v1/statistics/overview", s.statisticsOverview)
	router.HandleFunc("GET /api/v1/statistics/timeseries", s.statisticsTimeSeries)
	router.HandleFunc("GET /api/v1/statistics/outliers", s.statisticsOutliers)
	router.HandleFunc("GET /api/v1/transactions/extremes", s.transactionExtremes)
	router.HandleFunc("GET /api/v1/rentals/statistics/overview", s.rentalOverview)
	router.HandleFunc("GET /api/v1/rentals/statistics/timeseries", s.rentalTimeSeries)
	router.HandleFunc("GET /api/v1/rentals/statistics/outliers", s.rentalOutliers)
	router.HandleFunc("GET /api/v1/rentals/map/clusters", s.rentalMapClusters)
	router.HandleFunc("GET /api/v1/rentals/map/geographies", s.rentalMapGeographies)
	router.HandleFunc("GET /api/v1/rentals/map/properties", s.rentalMapProperties)
	router.HandleFunc("GET /api/v1/sales/transactions/{id}", s.saleTransaction)
	router.HandleFunc("GET /api/v1/sales/transactions/{id}/context", s.saleTransactionContext)
	router.HandleFunc("GET /api/v1/rentals/transactions/{id}", s.rentalTransaction)
	router.HandleFunc("GET /api/v1/rentals/transactions/{id}/context", s.rentalTransactionContext)
	router.HandleFunc("GET /api/v1/statistics/distribution", s.saleDistribution)
	router.HandleFunc("GET /api/v1/rentals/statistics/distribution", s.rentalDistribution)
	router.HandleFunc("GET /api/v1/buildings/{cadastralCode}/{buildingNumber}/statistics", s.buildingStatistics)
	router.HandleFunc("GET /api/v1/buildings/search", s.searchBuildings)
	router.HandleFunc("GET /api/v1/anomalies", s.anomalies)
	router.HandleFunc("GET /api/v1/anomalies/summary", s.anomalySummary)
	router.HandleFunc("PATCH /api/v1/anomalies/{kind}/{id}/review", s.reviewAnomaly)
	router.HandleFunc("POST /api/v1/auth/register", s.register)
	router.HandleFunc("POST /api/v1/auth/login", s.login)
	router.HandleFunc("POST /api/v1/auth/logout", s.logout)
	router.HandleFunc("GET /api/v1/users/me", s.me)
	router.HandleFunc("GET /api/v1/users/me/preferences", s.getPreferences)
	router.HandleFunc("PUT /api/v1/users/me/preferences", s.savePreferences)
	router.HandleFunc("GET /api/v1/indices", s.marketIndices)
	router.HandleFunc("GET /api/v1/valuations/{kind}/{id}", s.valuationPrediction)
	router.HandleFunc("GET /api/v1/map/clusters", s.mapClusters)
	router.HandleFunc("GET /api/v1/map/geographies", s.mapGeographies)
	router.HandleFunc("GET /api/v1/map/properties", s.mapProperties)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "route_not_found", "route was not found")
	})
	return router
}
