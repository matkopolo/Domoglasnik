package geography

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("geography not found")

type Unit struct {
	ID            int64     `db:"id" json:"id"`
	Level         string    `db:"level" json:"level"`
	Code          *int32    `db:"code" json:"code,omitempty"`
	CompositeCode *int32    `db:"composite_code" json:"compositeCode,omitempty"`
	Name          string    `db:"name" json:"name"`
	ParentID      *int64    `db:"parent_id" json:"parentId,omitempty"`
	GURSEID       string    `db:"gurs_eid" json:"gursEid"`
	Bounds        []float64 `db:"bounds" json:"bounds,omitempty"`
}

type Filter struct {
	Level    string
	ParentID *int64
}

type Store interface {
	List(context.Context, Filter) ([]Unit, error)
	Get(context.Context, int64) (Unit, error)
	Children(context.Context, int64) ([]Unit, error)
	Boundary(context.Context, int64, float64) (Boundary, error)
}

type Repository struct {
	pool  *pgxpool.Pool
	table string
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return NewRepositoryWithSchema(pool, "analytics")
}

func NewRepositoryWithSchema(pool *pgxpool.Pool, schema string) *Repository {
	return &Repository{
		pool:  pool,
		table: pgx.Identifier{schema, "geographic_units"}.Sanitize(),
	}
}

type Boundary struct {
	Unit     Unit            `json:"properties"`
	Geometry json.RawMessage `json:"geometry"`
}

func (r *Repository) List(ctx context.Context, filter Filter) ([]Unit, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, level, code, composite_code, name, parent_id, gurs_eid,
		       ARRAY[ST_XMin(extent), ST_YMin(extent), ST_XMax(extent), ST_YMax(extent)]::float8[] AS bounds
		FROM (
			SELECT id, level, code, composite_code, name, parent_id, gurs_eid,
			       ST_Envelope(geometry_wgs84) AS extent
			FROM %s
			WHERE ($1 = '' OR level = $1)
			  AND ($2::bigint IS NULL OR parent_id = $2)
		) units
		ORDER BY level, name, id`, r.table), filter.Level, filter.ParentID)
	if err != nil {
		return nil, fmt.Errorf("list geographies: %w", err)
	}
	defer rows.Close()

	units, err := pgx.CollectRows(rows, pgx.RowToStructByName[Unit])
	if err != nil {
		return nil, fmt.Errorf("collect geographies: %w", err)
	}
	return units, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (Unit, error) {
	row, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, level, code, composite_code, name, parent_id, gurs_eid,
		       ARRAY[ST_XMin(extent), ST_YMin(extent), ST_XMax(extent), ST_YMax(extent)]::float8[] AS bounds
		FROM (
			SELECT id, level, code, composite_code, name, parent_id, gurs_eid,
			       ST_Envelope(geometry_wgs84) AS extent
			FROM %s WHERE id=$1
		) unit`, r.table), id)
	if err != nil {
		return Unit{}, fmt.Errorf("get geography: %w", err)
	}
	unit, err := pgx.CollectOneRow(row, pgx.RowToStructByName[Unit])
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, ErrNotFound
	}
	if err != nil {
		return Unit{}, fmt.Errorf("collect geography: %w", err)
	}
	return unit, nil
}

func (r *Repository) Children(ctx context.Context, parentID int64) ([]Unit, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id=$1)", r.table), parentID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check geography: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	return r.List(ctx, Filter{ParentID: &parentID})
}

func (r *Repository) Boundary(ctx context.Context, id int64, tolerance float64) (Boundary, error) {
	unit, err := r.Get(ctx, id)
	if err != nil {
		return Boundary{}, err
	}
	var geometry json.RawMessage
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ST_AsGeoJSON(
			CASE WHEN $2::float8 > 0
			     THEN ST_SimplifyPreserveTopology(geometry_wgs84,$2)
			     ELSE geometry_wgs84 END,
			6
		)::jsonb
		FROM %s WHERE id=$1`, r.table), id, tolerance).Scan(&geometry)
	if err != nil {
		return Boundary{}, fmt.Errorf("get geography boundary: %w", err)
	}
	return Boundary{Unit: unit, Geometry: geometry}, nil
}
