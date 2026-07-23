package analytics

import (
	"context"
	"fmt"
)

type Bounds struct {
	West, South, East, North float64
}

type Cluster struct {
	Longitude           float64  `json:"longitude"`
	Latitude            float64  `json:"latitude"`
	TransactionCount    int64    `json:"transactionCount"`
	RepresentativeCount int64    `json:"representativeCount"`
	MedianPricePerSQM   *float64 `json:"medianPricePerSqm"`
}

type PropertyPoint struct {
	TransactionID    int64    `json:"transactionId"`
	Longitude        float64  `json:"longitude"`
	Latitude         float64  `json:"latitude"`
	ContractDate     *string  `json:"contractDate"`
	PropertyGroup    string   `json:"propertyGroup"`
	Area             *float64 `json:"area"`
	Price            *float64 `json:"price"`
	PricePerSQM      *float64 `json:"pricePerSqm"`
	IsRepresentative bool     `json:"isRepresentative"`
}

type MapStore interface {
	Clusters(context.Context, StatisticsFilter, Bounds, float64) ([]Cluster, error)
	PropertyPoints(context.Context, StatisticsFilter, Bounds, int) ([]PropertyPoint, error)
}

func (r *StatisticsRepository) Clusters(
	ctx context.Context,
	filter StatisticsFilter,
	bounds Bounds,
	gridSize float64,
) ([]Cluster, error) {
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		WITH bounded AS (
			SELECT
				ST_SnapToGrid(ST_Transform(location,4326),$9,$9) AS grid_point,
				price_per_sqm,representative_eligible
			FROM analytics.sale_facts
			WHERE location IS NOT NULL
			  AND location && ST_Transform(ST_MakeEnvelope($5,$6,$7,$8,4326),3794)
			  AND ($1::date IS NULL OR contract_date >= $1)
			  AND ($2::date IS NULL OR contract_date <= $2)
			  AND ($3 = '' OR property_group = $3)
			  AND %s
		)
		SELECT ST_X(grid_point),ST_Y(grid_point),count(*)::bigint,
		       count(*) FILTER (WHERE representative_eligible)::bigint,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm)
		           FILTER (WHERE representative_eligible)::float8
		FROM bounded
		GROUP BY grid_point
		ORDER BY count(*) DESC
		LIMIT 5000`, geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID,
		bounds.West, bounds.South, bounds.East, bounds.North, gridSize,
	)
	if err != nil {
		return nil, fmt.Errorf("map clusters: %w", err)
	}
	defer rows.Close()
	items := make([]Cluster, 0)
	for rows.Next() {
		var item Cluster
		if err := rows.Scan(
			&item.Longitude, &item.Latitude, &item.TransactionCount,
			&item.RepresentativeCount, &item.MedianPricePerSQM,
		); err != nil {
			return nil, fmt.Errorf("scan map cluster: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate map clusters: %w", err)
	}
	return items, nil
}

func (r *StatisticsRepository) PropertyPoints(
	ctx context.Context,
	filter StatisticsFilter,
	bounds Bounds,
	limit int,
) ([]PropertyPoint, error) {
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT transaction_id,ST_X(ST_Transform(location,4326)),
		       ST_Y(ST_Transform(location,4326)),contract_date::text,
		       property_group,area::float8,price::float8,price_per_sqm::float8,
		       representative_eligible
		FROM analytics.sale_facts
		WHERE location IS NOT NULL
		  AND location && ST_Transform(ST_MakeEnvelope($5,$6,$7,$8,4326),3794)
		  AND ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group = $3)
		  AND %s
		ORDER BY contract_date DESC NULLS LAST,transaction_id
		LIMIT $9`, geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID,
		bounds.West, bounds.South, bounds.East, bounds.North, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("map property points: %w", err)
	}
	defer rows.Close()
	items := make([]PropertyPoint, 0)
	for rows.Next() {
		var item PropertyPoint
		if err := rows.Scan(
			&item.TransactionID, &item.Longitude, &item.Latitude, &item.ContractDate,
			&item.PropertyGroup, &item.Area, &item.Price, &item.PricePerSQM,
			&item.IsRepresentative,
		); err != nil {
			return nil, fmt.Errorf("scan map property point: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate map property points: %w", err)
	}
	return items, nil
}
