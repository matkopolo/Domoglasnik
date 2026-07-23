package analytics

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSalePackageClassificationSQL(t *testing.T) {
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
	if _, err := tx.Exec(ctx, migrationSQL); err != nil {
		t.Fatalf("apply classification fixture: %v", err)
	}
	tests := []struct {
		buildings []int32
		land      []int32
		want      string
	}{
		{[]int32{1, 4}, []int32{5}, "house"},
		{[]int32{2, 3}, nil, "apartment"},
		{[]int32{1, 2}, nil, "mixed"},
		{nil, []int32{7}, "land"},
		{[]int32{4}, nil, "garage"},
	}
	for _, test := range tests {
		var got string
		err := tx.QueryRow(ctx,
			"SELECT analytics.classify_sale_package($1::integer[],$2::integer[])",
			test.buildings, test.land,
		).Scan(&got)
		if err != nil || got != test.want {
			t.Fatalf("classify(%v,%v)=%q error=%v want=%q",
				test.buildings, test.land, got, err, test.want)
		}
	}
}

const migrationSQL = `
CREATE SCHEMA IF NOT EXISTS analytics;
CREATE TABLE IF NOT EXISTS analytics.building_part_type_groups (
	source_code integer PRIMARY KEY,
	property_group text NOT NULL,
	is_primary boolean NOT NULL,
	description text NOT NULL
);
INSERT INTO analytics.building_part_type_groups(source_code,property_group,is_primary,description)
VALUES (1,'house',true,'house'),(2,'apartment',true,'apartment'),
       (3,'garage',false,'parking'),(4,'garage',false,'garage'),
       (8,'commercial',true,'commercial')
ON CONFLICT (source_code) DO UPDATE SET
	property_group=excluded.property_group,is_primary=excluded.is_primary;
CREATE OR REPLACE FUNCTION analytics.classify_sale_package(
	building_part_types integer[],
	land_types integer[]
)
RETURNS text LANGUAGE sql STABLE AS $$
WITH groups AS (
	SELECT DISTINCT m.property_group,m.is_primary
	FROM unnest(coalesce(building_part_types,ARRAY[]::integer[])) s(source_code)
	LEFT JOIN analytics.building_part_type_groups m USING(source_code)
), primary_groups AS (
	SELECT property_group FROM groups WHERE is_primary AND property_group <> 'unknown'
)
SELECT CASE
	WHEN (SELECT count(*) FROM primary_groups)>1 THEN 'mixed'
	WHEN (SELECT count(*) FROM primary_groups)=1 THEN (SELECT min(property_group) FROM primary_groups)
	WHEN EXISTS(SELECT 1 FROM groups WHERE property_group='garage') THEN 'garage'
	WHEN cardinality(coalesce(land_types,ARRAY[]::integer[]))>0 THEN 'land'
	ELSE 'unknown' END
$$;`
