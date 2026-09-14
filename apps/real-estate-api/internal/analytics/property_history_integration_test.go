package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPropertyHistoryIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO cadastral_municipalities(code,name) VALUES(999991,'History test'),(999992,'Other cadastral municipality') ON CONFLICT DO NOTHING")
	var source int64
	if err := tx.QueryRow(ctx, `INSERT INTO source_files(file_name,dataset_year,sha256) VALUES('history-test.zip',2025,md5(random()::text)||md5(random()::text)) RETURNING id`).Scan(&source); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		SELECT 8800000000000+i,DATE '2025-01-01'+i*4,100000+i*10000,2025,$1 FROM generate_series(0,9) i`, source)
	exec(`INSERT INTO building_parts(transaction_id,cadastral_municipality_code,building_number,building_part_number,building_part_type,sold_area,sold_share,dataset_year,source_file_id,source_row_number)
		SELECT 8800000000000+i,CASE WHEN i=9 THEN 999992 ELSE 999991 END,'999999','1',2,
		CASE WHEN i>=6 THEN 55 ELSE 50 END,CASE WHEN i=2 THEN '1/2' ELSE '1/1' END,2025,$1,i+1 FROM generate_series(0,9) i`, source)
	exec(`INSERT INTO analytics.sale_facts(transaction_id,contract_date,dataset_year,property_group,building_part_count,land_parcel_count,primary_property_count,area,price,price_per_sqm,price_derivation,is_quality_eligible,representative_eligible)
		SELECT transaction_id,contract_date,2025,'apartment',CASE WHEN transaction_id=8800000000007 THEN 2 ELSE 1 END,0,1,50,contract_price,contract_price/50,'single_primary_transaction_price',true,true
		FROM transactions WHERE transaction_id BETWEEN 8800000000000 AND 8800000000009`)
	exec(`INSERT INTO analytics.anomaly_signals(transaction_kind,transaction_id,flags,hard_rule_score,robust_cohort_score,duplicate_score,spatial_score,review_score,signal_version)
		VALUES('sale',8800000000004,ARRAY['possible_duplicate'],0,0,1,0,0.2,'test')`)
	exec(`INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		SELECT 8800000000010+i,DATE '2025-01-01'+i*10,10000+i*1000,2025,$1 FROM generate_series(0,3) i`, source)
	exec(`INSERT INTO analytics.sale_facts(transaction_id,contract_date,dataset_year,property_group,building_part_count,land_parcel_count,primary_property_count,area,price,price_per_sqm,price_derivation,is_quality_eligible,representative_eligible)
		SELECT transaction_id,contract_date,2025,'land',0,1,1,500,contract_price,contract_price/500,'single_primary_transaction_price',true,true
		FROM transactions WHERE transaction_id BETWEEN 8800000000010 AND 8800000000013`)

	// A parcel's source-code transition is not proof of zoning; geolocation-only
	// building records must never enter exact-identity price pairs.
	exec(`INSERT INTO land_parcels(transaction_id,cadastral_municipality_code,parcel_number,land_type,parcel_area,sold_share,dataset_year,source_file_id,source_row_number)
		VALUES (8800000000010,999991,'123/1',7,500,'1/1',2025,$1,30),
		       (8800000000011,999991,'123/1',1,500,'1/1',2025,$1,31),
		       (8800000000012,999991,'123/1',3,500,'1/1',2025,$1,32),
		       (8800000000013,999991,'123/1',3,500,'1/1',2025,$1,33),
		       (8800000000013,999991,'123/1',7,500,'1/1',2025,$1,34)`, source)
	exec(`UPDATE building_parts SET parcel_number_for_geolocation='123/1',construction_year=2025 WHERE transaction_id=8800000000008`)
	// A different analytical price must not replace the reported source contract price.
	exec(`UPDATE analytics.sale_facts SET price=777 WHERE transaction_id=8800000000001`)
	// Thirty consecutive pairs with known spacing exercise the actual summary SQL,
	// including the publication boundary and predecessors outside the date filter.
	exec(`INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		SELECT 8800000000100+i,DATE '1901-01-01'+i*10,100000+i*1000,1901,$1 FROM generate_series(0,30) i`, source)
	exec(`INSERT INTO building_parts(transaction_id,cadastral_municipality_code,building_number,building_part_number,building_part_type,sold_area,sold_share,dataset_year,source_file_id,source_row_number)
		SELECT 8800000000100+i,999991,'888888','1',2,60,'1/1',1901,$1,100+i FROM generate_series(0,30) i`, source)
	exec(`INSERT INTO analytics.sale_facts(transaction_id,contract_date,dataset_year,property_group,building_part_count,land_parcel_count,primary_property_count,area,price,price_per_sqm,price_derivation,is_quality_eligible,representative_eligible)
		SELECT transaction_id,contract_date,1901,'apartment',1,0,1,60,contract_price,contract_price/60,'single_primary_transaction_price',true,true
		FROM transactions WHERE transaction_id BETWEEN 8800000000100 AND 8800000000130`)
	exec(`SET LOCAL ROLE real_estate_api`)
	exec(`SET LOCAL max_parallel_workers_per_gather=0`)
	from := time.Date(1901, 1, 11, 0, 0, 0, 0, time.UTC)
	to := time.Date(1901, 12, 31, 0, 0, 0, 0, time.UTC)
	filter := StatisticsFilter{FromDate: &from, ToDate: &to, PropertyGroup: "apartment"}
	summary, err := loadResaleSummary(ctx, tx, filter, "$4::bigint IS NULL", nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PairCount != 30 || summary.AssetCount != 1 || !summary.IsPublishable || summary.MedianDays == nil || *summary.MedianDays != 10 || *summary.P25Days != 10 || *summary.P75Days != 10 {
		t.Fatalf("summary=%+v", summary)
	}
	from = from.AddDate(0, 0, 1)
	summary, err = loadResaleSummary(ctx, tx, filter, "$4::bigint IS NULL", nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PairCount != 29 || summary.IsPublishable || summary.MedianDays != nil || summary.P25Days != nil || summary.P75Days != nil {
		t.Fatalf("suppressed=%+v", summary)
	}
	min := 61.0
	filter.MinArea = &min
	summary, err = loadResaleSummary(ctx, tx, filter, "$4::bigint IS NULL", nil)
	if err != nil || summary.PairCount != 0 {
		t.Fatalf("area filter=%+v err=%v", summary, err)
	}
	history, err := loadPropertyHistory(ctx, tx, 8800000000000)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Assets) != 1 {
		t.Fatalf("assets=%+v", history.Assets)
	}
	for _, asset := range history.Assets {
		if asset.Kind == "building_part" {
			if len(asset.Events) != 9 {
				t.Fatalf("cross-cadastral identity leak: events=%d", len(asset.Events))
			}
			if len(asset.Pairs) != 8 {
				t.Fatalf("pairs=%d", len(asset.Pairs))
			}
			first := asset.Pairs[0]
			if !first.Comparable || first.Days == nil || *first.Days != 4 || first.NominalChange == nil || *first.NominalChange != 10000 {
				t.Fatalf("quick resale=%+v", first)
			}
			for _, pair := range asset.Pairs[1:] {
				if pair.Comparable || pair.NominalChange != nil || pair.ChangePercent != nil {
					t.Fatalf("ambiguous pair leaked price return: %+v", pair)
				}
			}
		}
	}
	land, err := loadPropertyHistory(ctx, tx, 8800000000010)
	if err != nil {
		t.Fatal(err)
	}
	if len(land.Assets) != 1 {
		t.Fatalf("land=%+v", land)
	}
	asset := land.Assets[0]
	if len(asset.Events) != 6 || len(asset.Pairs) != 3 || !asset.Pairs[0].LandTransition || asset.Pairs[0].Comparable {
		t.Fatalf("land evidence=%+v", asset)
	}
	for _, pair := range asset.Pairs[1:] {
		if pair.Comparable || pair.LandTransition {
			t.Fatalf("ambiguous/code 3 transition: %+v", pair)
		}
	}
	if _, err := loadPropertyHistory(ctx, tx, 8800000000099); err != ErrTransactionNotFound {
		t.Fatalf("missing=%v", err)
	}
}
