package geography

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryIntegration(t *testing.T) {
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

	schema := fmt.Sprintf("test_go_geography_%d", time.Now().UnixNano())
	qualified := pgx.Identifier{schema, "geographic_units"}.Sanitize()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %s;
		CREATE TABLE %s (
			id bigint PRIMARY KEY,
			level text NOT NULL,
			code integer,
			composite_code integer,
			name text NOT NULL,
			parent_id bigint,
			gurs_eid text NOT NULL,
			geometry_wgs84 geometry(MultiPolygon,4326) NOT NULL
		)`, pgx.Identifier{schema}.Sanitize(), qualified)); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	})

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s(id,level,code,name,gurs_eid,geometry_wgs84) VALUES
		(1,'country',1,'Slovenija','SI',ST_Multi(ST_GeomFromText('POLYGON((13 45,17 45,17 47,13 47,13 45))',4326))),
		(2,'statistical_region',1,'Test region','region-1',ST_Multi(ST_GeomFromText('POLYGON((14 46,15 46,15 46.5,14 46.5,14 46))',4326)));
		UPDATE %s SET parent_id=1 WHERE id=2`, qualified, qualified)); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	repository := NewRepositoryWithSchema(pool, schema)
	units, err := repository.List(ctx, Filter{Level: "statistical_region"})
	if err != nil || len(units) != 1 {
		t.Fatalf("List() units=%v error=%v", units, err)
	}
	unit, err := repository.Get(ctx, 2)
	if err != nil || unit.Name != "Test region" || len(unit.Bounds) != 4 {
		t.Fatalf("Get() unit=%+v error=%v", unit, err)
	}
	children, err := repository.Children(ctx, 1)
	if err != nil || len(children) != 1 || children[0].ID != 2 {
		t.Fatalf("Children() units=%v error=%v", children, err)
	}
	boundary, err := repository.Boundary(ctx, 2, 0.001)
	if err != nil || len(boundary.Geometry) == 0 {
		t.Fatalf("Boundary() boundary=%+v error=%v", boundary, err)
	}
	if _, err := repository.Get(ctx, 999); err != ErrNotFound {
		t.Fatalf("Get() missing error=%v", err)
	}
}
