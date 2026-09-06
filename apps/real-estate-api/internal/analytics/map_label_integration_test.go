package analytics

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLocalMapLabelsUseDistinctBuildingsAcrossMarkets(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// CTE fixtures shadow the source tables, leaving imported data untouched.
	// One rental repeats a sale location; the other location is outside the unit.
	query := fmt.Sprintf(`
		WITH building_parts AS (
		  SELECT ST_SetSRID(ST_Point(100,100),3794) AS location
		  FROM generate_series(1,$1::integer)
		  UNION ALL SELECT ST_SetSRID(ST_Point(300,100),3794)
		  WHERE $1::integer>0
		), rental_building_parts AS (
		  SELECT ST_SetSRID(ST_Point(300,100),3794) AS location WHERE $1::integer>0
		  UNION ALL SELECT ST_SetSRID(ST_Point(5000,5000),3794)
		), geography AS (
		  SELECT $2::text AS level, ST_GeomFromText($3,3794) AS geometry_d96tm
		), anchors AS (
		  SELECT unit.*,building_anchor.point AS label_anchor FROM geography unit %s
		), labels AS (
		  SELECT geometry_d96tm,ST_Transform(%s,3794) AS label FROM anchors unit
		)
		SELECT ST_Covers(geometry_d96tm,label),ST_X(label),ST_Y(label) FROM labels`,
		geographyBuildingAnchorJoin, geographyLabelPointExpression)
	square := "POLYGON((0 0,2000 0,2000 2000,0 2000,0 0))"
	for _, test := range []struct {
		name, level, polygon string
		repeats              int
		x, y                 float64
	}{
		{"buildings", "city_district", square, 1, 200, 100},
		{"repeated transactions have no extra weight", "city_district", square, 100, 200, 100},
		{"local community", "local_community", square, 1, 200, 100},
		{"settlement", "settlement", square, 1, 200, 100},
		{"no buildings falls back to geometry", "city_district", square, 0, 1000, 1000},
		{"broader geography keeps geometry anchor", "municipality", square, 100, 1000, 1000},
		{"tiny polygon has safe fallback", "city_district", "POLYGON((0 0,10 0,10 10,0 10,0 0))", 1, 5, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			var inside bool
			var x, y float64
			if err := pool.QueryRow(ctx, query, test.repeats, test.level, test.polygon).Scan(&inside, &x, &y); err != nil {
				t.Fatal(err)
			}
			if !inside || math.Abs(x-test.x) > 0.1 || math.Abs(y-test.y) > 0.1 {
				t.Fatalf("label=(%f,%f) inside=%v; want=(%f,%f) inside=true", x, y, inside, test.x, test.y)
			}
		})
	}
}
