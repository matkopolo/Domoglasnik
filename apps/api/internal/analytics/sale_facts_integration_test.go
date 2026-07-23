package analytics

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSaleFactsIntegration(t *testing.T) {
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

	var rawCount, factCount int64
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM transactions),
		       (SELECT count(*) FROM analytics.sale_facts)`,
	).Scan(&rawCount, &factCount); err != nil {
		t.Fatalf("inspect facts: %v", err)
	}
	if rawCount > 0 {
		if factCount != rawCount {
			t.Fatalf("facts=%d raw transactions=%d", factCount, rawCount)
		}
		assertSaleFactInvariants(t, ctx, pool)
		return
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin fixture: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sourceFileID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO source_files(file_name,dataset_year,dataset_type,dataset_scope,sha256)
		VALUES ('test-sales.zip',2024,'sales','national',repeat('a',64))
		RETURNING id`,
	).Scan(&sourceFileID)
	if err != nil {
		t.Fatalf("insert source file: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		VALUES (900000000001,DATE '2024-05-10',300000,2024,$1);
		INSERT INTO building_parts(
			transaction_id,building_part_type,sold_area,centroid_e,centroid_n,
			dataset_year,source_file_id,source_row_number
		) VALUES
			(900000000001,1,120,460000,100000,2024,$1,2),
			(900000000001,4,20,460005,100005,2024,$1,3);
		INSERT INTO land_parcels(
			transaction_id,land_type,parcel_area,centroid_e,centroid_n,
			dataset_year,source_file_id,source_row_number
		) VALUES (900000000001,5,500,460000,100000,2024,$1,4)`, sourceFileID)
	if err != nil {
		t.Fatalf("insert transaction fixture: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT analytics.refresh_sale_facts()"); err != nil {
		t.Fatalf("refresh facts: %v", err)
	}

	var group, derivation string
	var primaryCount int
	var area, pricePerSQM float64
	err = tx.QueryRow(ctx, `
		SELECT property_group,price_derivation,primary_property_count,
		       area::float8,price_per_sqm::float8
		FROM analytics.sale_facts WHERE transaction_id=900000000001`,
	).Scan(&group, &derivation, &primaryCount, &area, &pricePerSQM)
	if err != nil {
		t.Fatalf("read fixture fact: %v", err)
	}
	if group != "house" || derivation != "single_primary_transaction_price" ||
		primaryCount != 1 || area != 120 || math.Abs(pricePerSQM-2500) > 0.001 {
		t.Fatalf("unexpected fact group=%s derivation=%s primary=%d area=%f price/m2=%f",
			group, derivation, primaryCount, area, pricePerSQM)
	}
}

func assertSaleFactInvariants(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var invalidPricePerSQM, invalidSRID int64
	err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE price_per_sqm IS NOT NULL AND (price<=0 OR area<=0)),
			count(*) FILTER (WHERE location IS NOT NULL AND ST_SRID(location)<>3794)
		FROM analytics.sale_facts`,
	).Scan(&invalidPricePerSQM, &invalidSRID)
	if err != nil {
		t.Fatalf("check invariants: %v", err)
	}
	if invalidPricePerSQM != 0 || invalidSRID != 0 {
		t.Fatalf("invalid price/m2=%d invalid SRID=%d", invalidPricePerSQM, invalidSRID)
	}
}
