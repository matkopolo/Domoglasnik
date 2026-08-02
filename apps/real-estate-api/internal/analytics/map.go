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

func (r *StatisticsRepository) GeographySummaries(
	ctx context.Context,
	filter StatisticsFilter,
	bounds Bounds,
	level string,
	kind string,
) ([]GeographySummary, error) {
	table, metric := saleFactsTable(filter), "price_per_sqm"
	if kind == "rent" {
		table, metric = "analytics.rental_facts", "monthly_rent_per_sqm"
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
		SELECT unit.id,unit.level,unit.name,
		  ST_X(ST_PointOnSurface(unit.geometry_wgs84)),
		  ST_Y(ST_PointOnSurface(unit.geometry_wgs84)),
		  ARRAY[ST_XMin(extent),ST_YMin(extent),ST_XMax(extent),ST_YMax(extent)]::float8[],
		  count(*)::bigint,
		  count(*) FILTER (WHERE fact.representative_eligible)::bigint,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY fact.%s)
		    FILTER (WHERE fact.representative_eligible)::float8
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
		ORDER BY count(*) DESC`, metric, table, join, geographyClause)
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
			&item.MedianAmountPerSQM,
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
