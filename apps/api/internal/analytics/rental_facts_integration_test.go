package analytics

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRentalFactsIntegration(t *testing.T) {
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
	err = pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM rental_transactions),
		       (SELECT count(*) FROM analytics.rental_facts)`,
	).Scan(&rawCount, &factCount)
	if err != nil {
		t.Fatalf("inspect rental facts: %v", err)
	}
	if rawCount > 0 {
		if factCount != rawCount {
			t.Fatalf("rental facts=%d raw transactions=%d", factCount, rawCount)
		}
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
		VALUES ('test-rent.zip',2024,'rent','national',repeat('b',64))
		RETURNING id`,
	).Scan(&sourceFileID)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO rental_transactions(
			transaction_id,rental_transaction_type,contract_date,contract_rent,
			vat_included,rental_start_date,rental_time,rental_duration,
			operating_costs_included,dataset_year,source_file_id
		) VALUES (
			900000000002,1,DATE '2024-05-01',600,false,DATE '2024-06-01',
			'1','12',false,2024,$1
		);
		INSERT INTO rental_building_parts(
			transaction_id,rented_spaces_type,rented_spaces_area,
			centroid_e,centroid_n,dataset_year,source_file_id,source_row_number
		) VALUES (900000000002,2,50,460000,100000,2024,$1,2)`, sourceFileID)
	if err != nil {
		t.Fatalf("insert rental fixture: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT analytics.refresh_rental_facts()"); err != nil {
		t.Fatalf("refresh rental facts: %v", err)
	}
	var group, derivation, term string
	var duration int
	var rent, rentPerSQM float64
	var eligible bool
	err = tx.QueryRow(ctx, `
		SELECT property_group,rent_derivation,rental_term,duration_months,
		       monthly_rent_ex_vat::float8,monthly_rent_per_sqm::float8,
		       representative_eligible
		FROM analytics.rental_facts WHERE transaction_id=900000000002`,
	).Scan(&group, &derivation, &term, &duration, &rent, &rentPerSQM, &eligible)
	if err != nil {
		t.Fatalf("read rental fact: %v", err)
	}
	if group != "apartment" || derivation != "homogeneous_transaction_rent" ||
		term != "fixed" || duration != 12 || rent != 600 ||
		math.Abs(rentPerSQM-12) > 0.001 || !eligible {
		t.Fatalf("unexpected rental fact group=%s derivation=%s term=%s duration=%d rent=%f rent/m2=%f eligible=%v",
			group, derivation, term, duration, rent, rentPerSQM, eligible)
	}
}
