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
	MedianAmount        *float64 `json:"medianAmount,omitempty"`
}

type PropertyPoint struct {
	TransactionID             int64    `json:"transactionId"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode"`
	BuildingNumber            *string  `json:"buildingNumber"`
	Longitude                 float64  `json:"longitude"`
	Latitude                  float64  `json:"latitude"`
	ContractDate              *string  `json:"contractDate"`
	PropertyGroup             string   `json:"propertyGroup"`
	Area                      *float64 `json:"area"`
	Price                     *float64 `json:"price"`
	PricePerSQM               *float64 `json:"pricePerSqm"`
	IsRepresentative          bool     `json:"isRepresentative"`
}

type GeographySummary struct {
	GeographyID         int64     `json:"geographyId"`
	Level               string    `json:"level"`
	Name                string    `json:"name"`
	Longitude           float64   `json:"longitude"`
	Latitude            float64   `json:"latitude"`
	Bounds              []float64 `json:"bounds"`
	TransactionCount    int64     `json:"transactionCount"`
	RepresentativeCount int64     `json:"representativeCount"`
	MedianAmountPerSQM  *float64  `json:"medianAmountPerSqm"`
	MedianAmount        *float64  `json:"medianAmount"`
}

type MapStore interface {
	Clusters(context.Context, StatisticsFilter, Bounds, float64) ([]Cluster, error)
	PropertyPoints(context.Context, StatisticsFilter, Bounds, int) ([]PropertyPoint, error)
	GeographySummaries(context.Context, StatisticsFilter, Bounds, string, string) ([]GeographySummary, error)
}

var geographyFactColumns = map[string]string{
	"statistical_region": "statistical_region_id",
	"municipality":       "municipality_id",
	"city_district":      "city_district_id",
	"local_community":    "local_community_id",
	"settlement":         "settlement_id",
}

// Local administrative polygons can include large unbuilt extensions. Use
// distinct recorded building locations across both markets and all years to
// anchor their labels. UNION removes repeated sales/rentals at the same metre
// grid location. Current analytical filters and viewport never affect this anchor.
const geographyBuildingAnchorJoin = `LEFT JOIN LATERAL (
	SELECT ST_Centroid(ST_Collect(location)) AS point
	FROM (
		SELECT ST_SnapToGrid(part.location,1) AS location
		FROM building_parts part
		WHERE unit.level IN ('city_district','local_community','settlement')
		  AND part.location && unit.geometry_d96tm
		  AND ST_Covers(unit.geometry_d96tm,part.location)
		UNION
		SELECT ST_SnapToGrid(part.location,1) AS location
		FROM rental_building_parts part
		WHERE unit.level IN ('city_district','local_community','settlement')
		  AND part.location && unit.geometry_d96tm
		  AND ST_Covers(unit.geometry_d96tm,part.location)
	) locations
) building_anchor ON true`

// Project the preferred anchor into a 20-metre interior buffer. Tiny/narrow
// polygons with an empty buffer retain a guaranteed point on their surface.
const geographyLabelPointExpression = `ST_Transform(
	CASE
		WHEN ST_IsEmpty(ST_Buffer(unit.geometry_d96tm,-20))
			THEN ST_PointOnSurface(unit.geometry_d96tm)
		ELSE ST_ClosestPoint(
			ST_Buffer(unit.geometry_d96tm,-20),
			coalesce(unit.label_anchor,ST_Centroid(unit.geometry_d96tm))
		)
	END,
	4326
)`

func (r *StatisticsRepository) GeographySummaries(
	ctx context.Context,
	filter StatisticsFilter,
	bounds Bounds,
	level string,
	kind string,
) ([]GeographySummary, error) {
	table, metric, amountMetric := saleFactsTable(filter), "price_per_sqm", "price"
	if kind == "rent" {
		table, metric, amountMetric = "analytics.rental_facts", "monthly_rent_per_sqm", "monthly_rent_ex_vat"
	}
	join := "TRUE"
	if level != "country" {
		column, ok := geographyFactColumns[level]
		if !ok {
			return nil, fmt.Errorf("unsupported map geography level %q", level)
		}
		join = "fact." + column + "=unit.id"
	}
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		WITH summaries AS MATERIALIZED (
		SELECT unit.id,unit.level,unit.name,unit.geometry_d96tm,
		  ARRAY[ST_XMin(extent),ST_YMin(extent),ST_XMax(extent),ST_YMax(extent)]::float8[] AS bounds,
		  count(*)::bigint AS transaction_count,
		  count(*) FILTER (WHERE fact.representative_eligible)::bigint AS representative_count,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY fact.%s)
		    FILTER (WHERE fact.representative_eligible)::float8 AS median_per_sqm,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY fact.%s)
		    FILTER (WHERE fact.representative_eligible)::float8 AS median_amount
		FROM analytics.geographic_units unit
		JOIN %s fact ON %s
		CROSS JOIN LATERAL ST_Envelope(unit.geometry_wgs84) extent
		WHERE unit.level=$11
		  AND unit.geometry_wgs84 && ST_MakeEnvelope($7,$8,$9,$10,4326)
		  AND ($1::date IS NULL OR fact.contract_date >= $1)
		  AND ($2::date IS NULL OR fact.contract_date <= $2)
		  AND ($3 = '' OR fact.property_group=$3)
		  AND %s
		GROUP BY unit.id,unit.level,unit.name,unit.geometry_wgs84,extent
		), anchors AS MATERIALIZED (
		  SELECT unit.*, building_anchor.point AS label_anchor
		  FROM summaries unit %s
		), labels AS MATERIALIZED (
		  SELECT unit.*, %s AS label FROM anchors unit
		)
		SELECT id,level,name,ST_X(label),ST_Y(label),bounds,
		  transaction_count,representative_count,median_per_sqm,median_amount
		FROM labels ORDER BY transaction_count DESC,id`,
		metric, amountMetric, table, join, geographyClause,
		geographyBuildingAnchorJoin, geographyLabelPointExpression)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea,
		bounds.West, bounds.South, bounds.East, bounds.North, level,
	)
	if err != nil {
		return nil, fmt.Errorf("%s geography summaries: %w", kind, err)
	}
	defer rows.Close()
	items := make([]GeographySummary, 0)
	for rows.Next() {
		var item GeographySummary
		if err := rows.Scan(
			&item.GeographyID, &item.Level, &item.Name,
			&item.Longitude, &item.Latitude, &item.Bounds,
			&item.TransactionCount, &item.RepresentativeCount,
			&item.MedianAmountPerSQM, &item.MedianAmount,
		); err != nil {
			return nil, fmt.Errorf("scan geography summary: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
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
				ST_SnapToGrid(ST_Transform(location,4326),$11,$11) AS grid_point,
				price_per_sqm,representative_eligible
			FROM %s
			WHERE location IS NOT NULL
			  AND location && ST_Transform(ST_MakeEnvelope($7,$8,$9,$10,4326),3794)
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
		LIMIT 5000`, saleFactsTable(filter), geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea,
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
		SELECT transaction_id,cadastral_municipality_code,building_number,
		       ST_X(ST_Transform(location,4326)),
		       ST_Y(ST_Transform(location,4326)),contract_date::text,
		       property_group,area::float8,price::float8,price_per_sqm::float8,
		       representative_eligible
		FROM %s
		WHERE location IS NOT NULL
		  AND location && ST_Transform(ST_MakeEnvelope($7,$8,$9,$10,4326),3794)
		  AND ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group = $3)
		  AND %s
		ORDER BY contract_date DESC NULLS LAST,transaction_id
		LIMIT $11`, saleFactsTable(filter), geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea,
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
			&item.TransactionID, &item.CadastralMunicipalityCode,
			&item.BuildingNumber, &item.Longitude, &item.Latitude, &item.ContractDate,
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
