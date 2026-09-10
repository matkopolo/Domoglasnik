package analytics

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLandBrowsingCategoriesIntegration(t *testing.T) {
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
	tests := []struct {
		codes   []int32
		unknown int
		want    string
	}{
		{[]int32{1}, 0, "strict"}, {[]int32{2}, 0, "strict"}, {[]int32{1, 2}, 0, "strict"},
		{[]int32{3}, 0, "unverified"}, {[]int32{7}, 0, "agricultural"},
		{[]int32{8}, 0, "agricultural"}, {[]int32{7, 8}, 0, "agricultural"},
		{[]int32{9}, 0, "forest"}, {[]int32{7, 9}, 0, "mixed"},
		{[]int32{1, 7}, 0, "mixed"}, {[]int32{1, 3}, 0, "mixed"},
		{[]int32{7}, 1, "mixed"}, {[]int32{4, 5, 6, 10}, 0, "other"},
		{[]int32{999}, 0, "other"}, {nil, 1, "other"}, {nil, 0, "other"},
	}
	var source int64
	err = tx.QueryRow(ctx, `INSERT INTO source_files(file_name,dataset_year,dataset_type,dataset_scope,sha256)
	 VALUES('land-browsing-test.zip',2024,'sales','national',repeat('b',64)) RETURNING id`).Scan(&source)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(tests))
	for i, test := range tests {
		var got string
		if err := tx.QueryRow(ctx, `SELECT analytics.land_market_category($1::integer[],$2)`, test.codes, test.unknown).Scan(&got); err != nil || got != test.want {
			t.Fatalf("codes=%v unknown=%d category=%q want=%q err=%v", test.codes, test.unknown, got, test.want, err)
		}
		id := int64(9_223_372_036_854_774_100 + i)
		ids[i] = id
		_, err = tx.Exec(ctx, `INSERT INTO transactions(transaction_id,contract_date,contract_price,dataset_year,source_file_id)
		 VALUES($1,DATE '2024-01-01',10000,2024,$2)`, id, source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO analytics.sale_facts(transaction_id,contract_date,dataset_year,property_group,
		 building_part_count,land_parcel_count,primary_property_count,area,price,price_per_sqm,price_derivation,
		 is_quality_eligible,quality_reasons,representative_eligible)
		 VALUES($1,DATE '2024-01-01',2024,'land',0,1,1,100,10000,100,'component_prices',true,ARRAY[]::text[],true)`, id)
		if err != nil {
			t.Fatal(err)
		}
		// The final case deliberately has no classification row: it belongs to other.
		if i == len(tests)-1 {
			continue
		}
		_, err = tx.Exec(ctx, `INSERT INTO analytics.land_transaction_facts(transaction_id,parcel_count,land_type_codes,
		 readiness_class,is_strict_buildable,is_broad_buildable,strict_parcel_count,broad_parcel_count,unknown_parcel_count)
		 VALUES($1,1,coalesce($2::integer[],ARRAY[]::integer[]),'unknown',$3,$4,0,0,$5)`, id, test.codes,
			test.want == "strict", test.want == "strict" || test.want == "unverified" || i == 10, test.unknown)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range []string{"strict", "unverified", "agricultural", "forest", "mixed", "other", "all"} {
		query := fmt.Sprintf(`SELECT transaction_id FROM %s WHERE transaction_id=ANY($1::bigint[]) ORDER BY transaction_id`, saleFactsTable(StatisticsFilter{LandScope: scope}))
		rows, err := tx.Query(ctx, query, ids)
		if err != nil {
			t.Fatal(err)
		}
		got := map[int64]bool{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			got[id] = true
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		for i, test := range tests {
			want := scope == "all" || test.want == scope
			if got[ids[i]] != want {
				t.Fatalf("scope=%s codes=%v got membership=%v want=%v", scope, test.codes, got[ids[i]], want)
			}
		}
	}
	// API role can browse but gains no source-table write privileges.
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE real_estate_api"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM analytics.sale_facts_agricultural_land WHERE transaction_id=ANY($1::bigint[])`, ids).Scan(&count); err != nil || count != 3 {
		t.Fatalf("API agricultural count=%d err=%v", count, err)
	}
}
