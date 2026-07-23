package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/geography"
	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/identity"
)

type fakeDatabase struct{ err error }

func (database fakeDatabase) Ping(context.Context) error { return database.err }

type fakeGeographies struct {
	units []geography.Unit
	err   error
}

type fakeStatistics struct {
	overview analytics.Overview
	points   []analytics.TimeSeriesPoint
	err      error
}

type fakeMaps struct{}
type fakeTransactions struct{ err error }
type fakeDistributions struct{ err error }
type fakeBuildings struct{ err error }
type fakeAnomalies struct{ err error }
type fakeIdentities struct{ err error }
type fakeModels struct{ err error }

func (store fakeModels) MarketIndices(context.Context, analytics.MarketIndexFilter) ([]analytics.MarketIndexPoint, error) {
	return []analytics.MarketIndexPoint{{TransactionKind: "sale", Method: "hedonic"}}, store.err
}

func (store fakeModels) ValuationPrediction(context.Context, string, int64) (analytics.ValuationPrediction, error) {
	return analytics.ValuationPrediction{TransactionKind: "sale", TransactionID: -1}, store.err
}

func (store fakeIdentities) Register(context.Context, string, string, string, string) (identity.Session, error) {
	return identity.Session{Token: "token", User: identity.User{ID: "user-1"}}, store.err
}

func (store fakeIdentities) Login(context.Context, string, string, string) (identity.Session, error) {
	return identity.Session{Token: "token", User: identity.User{ID: "user-1"}}, store.err
}

func (store fakeIdentities) Authenticate(context.Context, string) (identity.User, error) {
	return identity.User{ID: "user-1", Email: "user@example.com"}, store.err
}

func (store fakeIdentities) Logout(context.Context, string) error { return store.err }

func (store fakeIdentities) Preferences(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"theme":"dark"}`), store.err
}

func (store fakeIdentities) SavePreferences(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"theme":"light"}`), store.err
}

func (store fakeAnomalies) Anomalies(
	context.Context,
	analytics.AnomalyFilter,
	int,
	int,
) ([]analytics.AnomalySignal, int64, error) {
	return []analytics.AnomalySignal{{TransactionKind: "sale", TransactionID: -1}}, 1, store.err
}

func (store fakeAnomalies) AnomalySummary(context.Context, string) (analytics.AnomalySummary, error) {
	return analytics.AnomalySummary{Total: 10, Flagged: 2}, store.err
}

func (store fakeAnomalies) ReviewAnomaly(context.Context, string, int64, string, *string, string) error {
	return store.err
}

func (store fakeBuildings) BuildingStatistics(context.Context, int32, string) (analytics.BuildingStatistics, error) {
	return analytics.BuildingStatistics{
		CadastralMunicipalityCode: 657,
		BuildingNumber:            "123",
	}, store.err
}

func (store fakeBuildings) SearchBuildings(
	context.Context, string, int,
) ([]analytics.BuildingSearchResult, error) {
	street, houseNumber := "LACKOVA CESTA", "13"
	return []analytics.BuildingSearchResult{{
		CadastralMunicipalityCode: 678,
		BuildingNumber:            "2952",
		Street:                    &street,
		HouseNumber:               &houseNumber,
		Longitude:                 15.61,
		Latitude:                  46.53,
		SaleCount:                 2,
	}}, store.err
}

func (store fakeDistributions) SaleDistribution(
	context.Context,
	analytics.StatisticsFilter,
	string,
	int,
) (analytics.Distribution, error) {
	return analytics.Distribution{Metric: "pricePerSqm", SampleSize: 10}, store.err
}

func (store fakeDistributions) RentalDistribution(
	context.Context,
	analytics.StatisticsFilter,
	string,
	int,
) (analytics.Distribution, error) {
	return analytics.Distribution{Metric: "rentPerSqm", SampleSize: 10}, store.err
}

func (store fakeTransactions) SaleTransaction(context.Context, int64) (analytics.SaleTransactionDetail, error) {
	return analytics.SaleTransactionDetail{TransactionID: 1, PropertyGroup: "apartment"}, store.err
}

func (store fakeTransactions) RentalTransaction(context.Context, int64) (analytics.RentalTransactionDetail, error) {
	return analytics.RentalTransactionDetail{TransactionID: 1, PropertyGroup: "apartment"}, store.err
}

func (fakeMaps) Clusters(
	context.Context,
	analytics.StatisticsFilter,
	analytics.Bounds,
	float64,
) ([]analytics.Cluster, error) {
	return []analytics.Cluster{{Longitude: 15.6, Latitude: 46.5, TransactionCount: 10}}, nil
}

func (fakeMaps) PropertyPoints(
	context.Context,
	analytics.StatisticsFilter,
	analytics.Bounds,
	int,
) ([]analytics.PropertyPoint, error) {
	return []analytics.PropertyPoint{{
		TransactionID: 1,
		Longitude:     15.6,
		Latitude:      46.5,
		PropertyGroup: "apartment",
	}}, nil
}

func (fakeMaps) GeographySummaries(
	context.Context,
	analytics.StatisticsFilter,
	analytics.Bounds,
	string,
	string,
) ([]analytics.GeographySummary, error) {
	return []analytics.GeographySummary{{
		GeographyID: 1, Level: "country", Name: "Slovenija",
		Longitude: 14.8, Latitude: 46.1, Bounds: []float64{13.3, 45.4, 16.7, 46.9},
		TransactionCount: 10,
	}}, nil
}

func (store fakeStatistics) Overview(
	context.Context,
	analytics.StatisticsFilter,
) (analytics.Overview, error) {
	return store.overview, store.err
}

func (store fakeStatistics) TimeSeries(
	context.Context,
	analytics.StatisticsFilter,
	string,
) ([]analytics.TimeSeriesPoint, error) {
	return store.points, store.err
}

func (store fakeStatistics) Outliers(
	context.Context,
	analytics.StatisticsFilter,
	int,
	int,
) ([]analytics.OutlierRecord, int64, error) {
	return []analytics.OutlierRecord{{TransactionID: 1, PropertyGroup: "apartment"}}, 1, store.err
}

func (store fakeStatistics) RentalOverview(
	context.Context,
	analytics.StatisticsFilter,
) (analytics.RentalOverview, error) {
	return analytics.RentalOverview{TransactionCount: 10, IncludedSampleSize: 8}, store.err
}

func (store fakeStatistics) RentalTimeSeries(
	context.Context,
	analytics.StatisticsFilter,
	string,
) ([]analytics.RentalTimeSeriesPoint, error) {
	return []analytics.RentalTimeSeriesPoint{{Period: "2024-01-01", TransactionCount: 10}}, store.err
}

func (store fakeStatistics) RentalOutliers(
	context.Context,
	analytics.StatisticsFilter,
	int,
	int,
) ([]analytics.RentalOutlierRecord, int64, error) {
	return []analytics.RentalOutlierRecord{{TransactionID: 1, PropertyGroup: "apartment"}}, 1, store.err
}

func (store fakeStatistics) RentalClusters(
	context.Context,
	analytics.StatisticsFilter,
	analytics.Bounds,
	float64,
) ([]analytics.Cluster, error) {
	return []analytics.Cluster{{Longitude: 15.6, Latitude: 46.5}}, store.err
}

func (store fakeStatistics) RentalPropertyPoints(
	context.Context,
	analytics.StatisticsFilter,
	analytics.Bounds,
	int,
) ([]analytics.RentalPropertyPoint, error) {
	return []analytics.RentalPropertyPoint{{TransactionID: 1, Longitude: 15.6, Latitude: 46.5}}, store.err
}

func (store fakeGeographies) List(context.Context, geography.Filter) ([]geography.Unit, error) {
	return store.units, store.err
}

func (store fakeGeographies) Get(context.Context, int64) (geography.Unit, error) {
	if store.err != nil {
		return geography.Unit{}, store.err
	}
	return store.units[0], nil
}

func (store fakeGeographies) Children(context.Context, int64) ([]geography.Unit, error) {
	return store.units, store.err
}

func (store fakeGeographies) Boundary(context.Context, int64, float64) (geography.Boundary, error) {
	if store.err != nil {
		return geography.Boundary{}, store.err
	}
	return geography.Boundary{
		Unit:     store.units[0],
		Geometry: []byte(`{"type":"Polygon","coordinates":[]}`),
	}, nil
}

func newTestHandler(databaseError error, store fakeGeographies) http.Handler {
	return New(Dependencies{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Database:    fakeDatabase{err: databaseError},
		Geographies: store,
		Statistics: fakeStatistics{
			overview: analytics.Overview{TransactionCount: 10, EligibleCount: 8},
			points:   []analytics.TimeSeriesPoint{{Period: "2024-01-01", TransactionCount: 10}},
		},
		Rentals:          fakeStatistics{},
		Transactions:     fakeTransactions{},
		Distributions:    fakeDistributions{},
		Buildings:        fakeBuildings{},
		Anomalies:        fakeAnomalies{},
		Identities:       fakeIdentities{},
		Models:           fakeModels{},
		Maps:             fakeMaps{},
		ApplicationName:  "test-api",
		ApplicationBuild: "test",
		Environment:      "test",
	})
}
