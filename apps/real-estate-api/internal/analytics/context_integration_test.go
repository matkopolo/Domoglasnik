package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCurrentMarketCandidatesUseCompositionAndExcludeNearDuplicates(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const subjectID int64 = 1900000000
	var sourceFileID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO source_files(file_name,dataset_year,dataset_type,dataset_scope,sha256)
		VALUES ('context-composition-integration.zip',extract(year FROM current_date)::integer,
		        'sales','national',repeat('c',64))
		RETURNING id`).Scan(&sourceFileID)
	if err != nil {
		t.Fatalf("insert source file: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		SELECT $1+seq,current_date-(seq||' days')::interval,150000+seq*1000,
		       extract(year FROM current_date)::integer,$2
		FROM generate_series(0,6) seq
		ON CONFLICT(transaction_id) DO NOTHING`, subjectID, sourceFileID)
	if err != nil {
		t.Fatalf("insert transactions: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO building_parts(
		  transaction_id,building_part_type,sold_area,centroid_e,centroid_n,
		  dataset_year,source_file_id,source_row_number
		)
		SELECT $1+seq,type_code,50,460000+seq*10,100000,
		       extract(year FROM current_date)::integer,$2,100+seq*10+type_code
		FROM generate_series(0,6) seq
		CROSS JOIN LATERAL unnest(
		  CASE WHEN seq=6 THEN ARRAY[2,14] ELSE ARRAY[2,3] END
		) type_code`, subjectID, sourceFileID)
	if err != nil {
		t.Fatalf("insert package components: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO analytics.sale_facts(
		  transaction_id,contract_date,dataset_year,property_group,
		  building_part_count,land_parcel_count,primary_property_count,
		  area,price,price_per_sqm,price_derivation,is_quality_eligible,
		  representative_eligible,quality_reasons,location,construction_year
		  ,cadastral_municipality_code,building_number
		)
		SELECT $1+seq,current_date-(seq||' days')::interval,
		       extract(year FROM current_date)::integer,'apartment',2,0,1,
		       50,150000+seq*1000,(150000+seq*1000)/50.0,
		       'single_primary_transaction_price',true,true,ARRAY[]::text[],
		       ST_SetSRID(ST_MakePoint(460000+seq*10,100000),3794),2000,9999,'77'
		FROM generate_series(0,6) seq
		ON CONFLICT(transaction_id) DO UPDATE SET representative_eligible=true`, subjectID)
	if err != nil {
		t.Fatalf("insert facts: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO analytics.near_duplicate_candidates(
		  transaction_kind,transaction_id,candidate_transaction_id,identity_basis,
		  days_apart,relative_area_difference,relative_amount_difference,
		  candidate_score,rule_version
		) VALUES ('sale',$1::bigint,$1::bigint+1,'same_building_part',1,0,0,.99,'test')
		ON CONFLICT DO NOTHING`, subjectID)
	if err != nil {
		t.Fatalf("insert near duplicate: %v", err)
	}

	var signature string
	if err := tx.QueryRow(ctx,
		"SELECT analytics.sale_package_signature($1)", subjectID).Scan(&signature); err != nil {
		t.Fatalf("load signature: %v", err)
	}
	candidates, err := queryCurrentCandidates(ctx, tx, currentMarketQuery{
		table: "analytics.sale_facts", subjectTable: "analytics.sale_facts",
		amountColumn:  "price_per_sqm",
		propertyGroup: "apartment", transactionKind: "sale", subjectID: subjectID,
		area: 50, constructionYear: intPointer(2000),
		packageSignature: signature,
	}, nearbyBenchmarkProfiles(intPointer(2000))[0], 1000,
		time.Now().Year(), true)
	if err != nil {
		t.Fatalf("query candidates: %v", err)
	}
	if len(candidates) != 4 {
		t.Fatalf("candidate count=%d want=4 (subject, duplicate and mismatched composition excluded)",
			len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.packageSignature != signature {
			t.Fatalf("composition mismatch: got %q want %q", candidate.packageSignature, signature)
		}
	}
	direct, err := loadDirectComparables(ctx, tx, directComparableQuery{
		table: "analytics.sale_facts", amountColumn: "price_per_sqm",
		totalColumn: "price", outlierTable: "analytics.sale_outlier_evaluations",
		propertyGroup: "apartment", buildingNumber: "77", transactionKind: "sale",
		subjectID: subjectID, actual: 3000, area: 50, year: time.Now().Year(),
		cadastralCode: 9999,
	})
	if err != nil {
		t.Fatalf("query direct comparables: %v", err)
	}
	if len(direct) != 5 {
		t.Fatalf("direct comparable count=%d want=5 (subject and duplicate excluded)", len(direct))
	}
}

func intPointer(value int) *int { return &value }
