package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStatisticsRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var factCount int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM analytics.sale_facts").Scan(&factCount); err != nil {
		t.Fatalf("count facts: %v", err)
	}
	if factCount == 0 {
		t.Skip("sale facts are empty; synthetic refresh is covered by TestSaleFactsIntegration")
	}

	repository := NewStatisticsRepository(pool)
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	overview, err := repository.Overview(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	})
	if err != nil {
		t.Fatalf("Overview(): %v", err)
	}
	if overview.TransactionCount == 0 || overview.EligibleCount == 0 ||
		overview.MedianPricePerSQM == nil {
		t.Fatalf("Overview()=%+v", overview)
	}

	points, err := repository.TimeSeries(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, "month")
	if err != nil {
		t.Fatalf("TimeSeries(): %v", err)
	}
	if len(points) != 12 || points[0].Period != "2023-01-01" {
		t.Fatalf("TimeSeries() first=%+v count=%d", points[0], len(points))
	}
	outliers, total, err := repository.Outliers(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, 2, 0)
	if err != nil || total == 0 || len(outliers) != 2 {
		t.Fatalf("Outliers() total=%d items=%d error=%v", total, len(outliers), err)
	}

	rentalOverview, err := repository.RentalOverview(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	})
	if err != nil || rentalOverview.TransactionCount == 0 ||
		rentalOverview.MedianMonthlyRentPerSQM == nil {
		t.Fatalf("RentalOverview()=%+v error=%v", rentalOverview, err)
	}
	rentalFrom := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	rentalPoints, err := repository.RentalTimeSeries(ctx, StatisticsFilter{
		FromDate: &rentalFrom, ToDate: &to, PropertyGroup: "apartment",
	}, "year")
	if err != nil || len(rentalPoints) != 2 ||
		rentalPoints[1].RentPerSQMYoYPercent == nil {
		t.Fatalf("RentalTimeSeries()=%+v error=%v", rentalPoints, err)
	}
	rentalOutliers, rentalTotal, err := repository.RentalOutliers(
		ctx, StatisticsFilter{FromDate: &from, ToDate: &to}, 2, 0)
	if err != nil || rentalTotal == 0 || len(rentalOutliers) != 2 {
		t.Fatalf("RentalOutliers() total=%d items=%d error=%v",
			rentalTotal, len(rentalOutliers), err)
	}
	rentalClusters, err := repository.RentalClusters(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, Bounds{West: 15.55, South: 46.52, East: 15.75, North: 46.62}, 0.03)
	if err != nil || len(rentalClusters) == 0 {
		t.Fatalf("RentalClusters() count=%d error=%v", len(rentalClusters), err)
	}
	rentalProperties, err := repository.RentalPropertyPoints(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, Bounds{West: 15.55, South: 46.52, East: 15.75, North: 46.62}, 5)
	if err != nil || len(rentalProperties) == 0 || len(rentalProperties) > 5 {
		t.Fatalf("RentalPropertyPoints() count=%d error=%v",
			len(rentalProperties), err)
	}

	var saleID, rentalID int64
	if err := pool.QueryRow(ctx, `
		SELECT transaction_id FROM analytics.sale_facts
		WHERE building_part_count>0 AND land_parcel_count>0 LIMIT 1`).Scan(&saleID); err != nil {
		t.Fatalf("select sale detail fixture: %v", err)
	}
	saleDetail, err := repository.SaleTransaction(ctx, saleID)
	if err != nil || saleDetail.TransactionID != saleID ||
		len(saleDetail.BuildingParts) == 0 || len(saleDetail.LandParcels) == 0 {
		t.Fatalf("SaleTransaction()=%+v error=%v", saleDetail, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT transaction_id FROM analytics.rental_facts
		WHERE space_count>0 LIMIT 1`).Scan(&rentalID); err != nil {
		t.Fatalf("select rental detail fixture: %v", err)
	}
	rentalDetail, err := repository.RentalTransaction(ctx, rentalID)
	if err != nil || rentalDetail.TransactionID != rentalID ||
		len(rentalDetail.Spaces) == 0 {
		t.Fatalf("RentalTransaction()=%+v error=%v", rentalDetail, err)
	}
	if _, err := repository.SaleTransaction(ctx, 999999999999); err != ErrTransactionNotFound {
		t.Fatalf("missing sale transaction error=%v", err)
	}
	saleDistribution, err := repository.SaleDistribution(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, "pricePerSqm", 10)
	if err != nil || saleDistribution.SampleSize == 0 ||
		saleDistribution.Percentiles == nil || len(saleDistribution.Buckets) != 10 {
		t.Fatalf("SaleDistribution()=%+v error=%v", saleDistribution, err)
	}
	rentalDistribution, err := repository.RentalDistribution(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, "rentPerSqm", 10)
	if err != nil || rentalDistribution.SampleSize == 0 ||
		rentalDistribution.Percentiles == nil || len(rentalDistribution.Buckets) != 10 {
		t.Fatalf("RentalDistribution()=%+v error=%v", rentalDistribution, err)
	}
	var cadastralCode int32
	var buildingNumber string
	if err := pool.QueryRow(ctx, `
		SELECT cadastral_municipality_code,building_number
		FROM analytics.sale_facts
		WHERE cadastral_municipality_code IS NOT NULL
		  AND building_number IS NOT NULL LIMIT 1`).Scan(
		&cadastralCode, &buildingNumber); err != nil {
		t.Fatalf("select building fixture: %v", err)
	}
	building, err := repository.BuildingStatistics(ctx, cadastralCode, buildingNumber)
	if err != nil || building.BuildingNumber != buildingNumber ||
		building.Sales.TransactionCount == 0 {
		t.Fatalf("BuildingStatistics()=%+v error=%v", building, err)
	}
	if _, err := repository.BuildingStatistics(ctx, 999999, "missing"); err != ErrBuildingNotFound {
		t.Fatalf("missing building error=%v", err)
	}
	anomalies, anomalyTotal, err := repository.Anomalies(ctx, AnomalyFilter{
		TransactionKind: "sale", MinimumScore: 0.2,
	}, 5, 0)
	if err != nil || anomalyTotal == 0 || len(anomalies) != 5 {
		t.Fatalf("Anomalies() total=%d count=%d error=%v",
			anomalyTotal, len(anomalies), err)
	}
	anomalySummary, err := repository.AnomalySummary(ctx, "rent")
	if err != nil || anomalySummary.Total == 0 || anomalySummary.Flagged == 0 {
		t.Fatalf("AnomalySummary()=%+v error=%v", anomalySummary, err)
	}
	indices, err := repository.MarketIndices(ctx, MarketIndexFilter{
		TransactionKind: "sale", Method: "hedonic", PropertyGroup: "apartment",
	})
	if err != nil || len(indices) == 0 || indices[0].IndexValue <= 0 {
		t.Fatalf("MarketIndices() count=%d error=%v", len(indices), err)
	}
	var predictionID int64
	if err := pool.QueryRow(ctx, `
		SELECT transaction_id FROM analytics.valuation_predictions
		WHERE transaction_kind='sale' LIMIT 1`).Scan(&predictionID); err != nil {
		t.Fatalf("select prediction fixture: %v", err)
	}
	prediction, err := repository.ValuationPrediction(ctx, "sale", predictionID)
	if err != nil || prediction.PredictedAmountPerSQM <= 0 ||
		prediction.LowerAmountPerSQM >= prediction.UpperAmountPerSQM {
		t.Fatalf("ValuationPrediction()=%+v error=%v", prediction, err)
	}
	bounds := Bounds{West: 15.55, South: 46.52, East: 15.75, North: 46.62}
	clusters, err := repository.Clusters(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, bounds, 0.03)
	if err != nil || len(clusters) == 0 {
		t.Fatalf("Clusters() count=%d error=%v", len(clusters), err)
	}
	properties, err := repository.PropertyPoints(ctx, StatisticsFilter{
		FromDate: &from, ToDate: &to, PropertyGroup: "apartment",
	}, bounds, 5)
	if err != nil || len(properties) == 0 || len(properties) > 5 {
		t.Fatalf("PropertyPoints() count=%d error=%v", len(properties), err)
	}

	missingID := int64(999999999)
	if _, err := repository.Overview(ctx, StatisticsFilter{GeographyID: &missingID}); err != ErrGeographyNotFound {
		t.Fatalf("missing geography error=%v", err)
	}
}

func TestGeographySummariesIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	repository := NewStatisticsRepository(pool)
	items, err := repository.GeographySummaries(
		context.Background(),
		StatisticsFilter{PropertyGroup: "apartment"},
		Bounds{West: 13.3, South: 45.4, East: 16.7, North: 46.9},
		"statistical_region",
		"sale",
	)
	if err != nil {
		t.Fatalf("GeographySummaries(): %v", err)
	}
	if len(items) != 12 {
		t.Fatalf("GeographySummaries() count=%d want=12", len(items))
	}
	for _, item := range items {
		if item.Name == "" || item.TransactionCount == 0 || len(item.Bounds) != 4 {
			t.Fatalf("invalid geography summary: %+v", item)
		}
	}
}
