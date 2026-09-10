package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrGeographyNotFound = errors.New("geography not found")

const descriptivePublicationThreshold int64 = 30

type StatisticsFilter struct {
	FromDate      *time.Time
	ToDate        *time.Time
	PropertyGroup string
	GeographyID   *int64
	LandScope     string
	MinArea       *float64
	MaxArea       *float64
}

func saleFactsTable(filter StatisticsFilter) string {
	switch filter.LandScope {
	case "broad":
		return "analytics.sale_facts_broad_land"
	case "all":
		return "analytics.sale_facts_all_land"
	case "agricultural":
		return "analytics.sale_facts_agricultural_land"
	case "forest":
		return "analytics.sale_facts_forest_land"
	case "unverified":
		return "analytics.sale_facts_unverified_land"
	case "mixed":
		return "analytics.sale_facts_mixed_land"
	case "other":
		return "analytics.sale_facts_other_land"
	default:
		return "analytics.sale_facts_strict_land"
	}
}

type Overview struct {
	TransactionCount      int64    `json:"transactionCount"`
	EligibleCount         int64    `json:"eligibleCount"`
	IncludedSampleSize    int64    `json:"includedSampleSize"`
	ExcludedOutlierCount  int64    `json:"excludedOutlierCount"`
	PriceSampleSize       int64    `json:"priceSampleSize"`
	PricePerSQMSampleSize int64    `json:"pricePerSqmSampleSize"`
	MedianPrice           *float64 `json:"medianPrice"`
	CleanedAveragePrice   *float64 `json:"cleanedAveragePrice"`
	RawAveragePrice       *float64 `json:"rawAveragePrice"`
	MedianPricePerSQM     *float64 `json:"medianPricePerSqm"`
	CleanedAveragePerSQM  *float64 `json:"cleanedAveragePricePerSqm"`
	RawAveragePerSQM      *float64 `json:"rawAveragePricePerSqm"`
	MedianArea            *float64 `json:"medianArea"`
	IsPublishable         bool     `json:"isPublishable"`
	SuppressionReason     *string  `json:"suppressionReason"`
}

type TimeSeriesPoint struct {
	Period                string   `json:"period"`
	Incomplete            bool     `json:"incomplete"`
	TransactionCount      int64    `json:"transactionCount"`
	EligibleCount         int64    `json:"eligibleCount"`
	IncludedSampleSize    int64    `json:"includedSampleSize"`
	ExcludedOutlierCount  int64    `json:"excludedOutlierCount"`
	PriceSampleSize       int64    `json:"priceSampleSize"`
	PricePerSQMSampleSize int64    `json:"pricePerSqmSampleSize"`
	MedianPrice           *float64 `json:"medianPrice"`
	CleanedAveragePrice   *float64 `json:"cleanedAveragePrice"`
	RawAveragePrice       *float64 `json:"rawAveragePrice"`
	MedianPricePerSQM     *float64 `json:"medianPricePerSqm"`
	CleanedAveragePerSQM  *float64 `json:"cleanedAveragePricePerSqm"`
	RawAveragePerSQM      *float64 `json:"rawAveragePricePerSqm"`
	MedianArea            *float64 `json:"medianArea"`
	IsPublishable         bool     `json:"isPublishable"`
	SuppressionReason     *string  `json:"suppressionReason"`
}

type StatisticsStore interface {
	Overview(context.Context, StatisticsFilter) (Overview, error)
	TimeSeries(context.Context, StatisticsFilter, string) ([]TimeSeriesPoint, error)
	Outliers(context.Context, StatisticsFilter, int, int) ([]OutlierRecord, int64, error)
	TransactionExtremes(context.Context, string, StatisticsFilter, int) ([]TransactionExtreme, error)
}

type OutlierRecord struct {
	TransactionID             int64    `json:"transactionId"`
	ContractDate              *string  `json:"contractDate"`
	PropertyGroup             string   `json:"propertyGroup"`
	Area                      *float64 `json:"area"`
	Price                     *float64 `json:"price"`
	PricePerSQM               *float64 `json:"pricePerSqm"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode,omitempty"`
	BuildingNumber            *string  `json:"buildingNumber,omitempty"`
	BuildingPartNumber        *string  `json:"buildingPartNumber,omitempty"`
	CohortLevel               string   `json:"cohortLevel"`
	CohortSize                int      `json:"cohortSize"`
	CohortMedian              float64  `json:"cohortMedian"`
	LowerBound                float64  `json:"lowerBound"`
	UpperBound                float64  `json:"upperBound"`
	ModifiedZScore            *float64 `json:"modifiedZScore"`
	RuleVersion               string   `json:"ruleVersion"`
}

type StatisticsRepository struct {
	pool *pgxpool.Pool
}

func NewStatisticsRepository(pool *pgxpool.Pool) *StatisticsRepository {
	return &StatisticsRepository{pool: pool}
}

func (r *StatisticsRepository) Overview(ctx context.Context, filter StatisticsFilter) (Overview, error) {
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return Overview{}, err
	}
	query := fmt.Sprintf(`
		SELECT
			count(*)::bigint,
			count(*) FILTER (WHERE is_quality_eligible)::bigint,
			count(*) FILTER (WHERE representative_eligible)::bigint,
			count(*) FILTER (WHERE is_quality_eligible AND NOT representative_eligible)::bigint,
			count(price) FILTER (WHERE representative_eligible)::bigint,
			count(price_per_sqm) FILTER (WHERE representative_eligible)::bigint,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price)
				FILTER (WHERE representative_eligible)::float8,
			avg(price) FILTER (WHERE representative_eligible)::float8,
			avg(price) FILTER (WHERE is_quality_eligible)::float8,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm)
				FILTER (WHERE representative_eligible)::float8,
			avg(price_per_sqm) FILTER (WHERE representative_eligible)::float8,
			avg(price_per_sqm) FILTER (WHERE is_quality_eligible)::float8,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY area)
				FILTER (WHERE representative_eligible)::float8
		FROM %s
		WHERE ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group = $3)
		  AND %s`, saleFactsTable(filter), geographyClause)
	var result Overview
	err = r.pool.QueryRow(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea,
	).Scan(
		&result.TransactionCount, &result.EligibleCount,
		&result.IncludedSampleSize, &result.ExcludedOutlierCount,
		&result.PriceSampleSize, &result.PricePerSQMSampleSize,
		&result.MedianPrice, &result.CleanedAveragePrice, &result.RawAveragePrice,
		&result.MedianPricePerSQM, &result.CleanedAveragePerSQM, &result.RawAveragePerSQM,
		&result.MedianArea,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("overview statistics: %w", err)
	}
	applyPublicationPolicy(result.IncludedSampleSize,
		&result.IsPublishable, &result.SuppressionReason)
	return result, nil
}

func (r *StatisticsRepository) TimeSeries(
	ctx context.Context,
	filter StatisticsFilter,
	interval string,
) ([]TimeSeriesPoint, error) {
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	truncUnit, step := intervalSQL(interval)
	query := fmt.Sprintf(`
		SELECT
			date_trunc('%s',contract_date)::date,
			(date_trunc('%s',contract_date)+interval '%s'>current_date),
			count(*)::bigint,
			count(*) FILTER (WHERE is_quality_eligible)::bigint,
			count(*) FILTER (WHERE representative_eligible)::bigint,
			count(*) FILTER (WHERE is_quality_eligible AND NOT representative_eligible)::bigint,
			count(price) FILTER (WHERE representative_eligible)::bigint,
			count(price_per_sqm) FILTER (WHERE representative_eligible)::bigint,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price)
				FILTER (WHERE representative_eligible)::float8,
			avg(price) FILTER (WHERE representative_eligible)::float8,
			avg(price) FILTER (WHERE is_quality_eligible)::float8,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm)
				FILTER (WHERE representative_eligible)::float8,
			avg(price_per_sqm) FILTER (WHERE representative_eligible)::float8,
			avg(price_per_sqm) FILTER (WHERE is_quality_eligible)::float8,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY area)
				FILTER (WHERE representative_eligible)::float8
		FROM %s
		WHERE contract_date IS NOT NULL
		  AND ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group = $3)
		  AND %s
		GROUP BY 1,2 ORDER BY 1`, truncUnit, truncUnit, step,
		saleFactsTable(filter), geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea,
	)
	if err != nil {
		return nil, fmt.Errorf("time-series statistics: %w", err)
	}
	defer rows.Close()
	points := make([]TimeSeriesPoint, 0)
	for rows.Next() {
		var period time.Time
		var point TimeSeriesPoint
		if err := rows.Scan(
			&period, &point.Incomplete, &point.TransactionCount,
			&point.EligibleCount, &point.IncludedSampleSize, &point.ExcludedOutlierCount,
			&point.PriceSampleSize, &point.PricePerSQMSampleSize,
			&point.MedianPrice, &point.CleanedAveragePrice, &point.RawAveragePrice,
			&point.MedianPricePerSQM, &point.CleanedAveragePerSQM, &point.RawAveragePerSQM,
			&point.MedianArea,
		); err != nil {
			return nil, fmt.Errorf("scan time-series statistics: %w", err)
		}
		point.Period = period.Format("2006-01-02")
		applyPublicationPolicy(point.IncludedSampleSize,
			&point.IsPublishable, &point.SuppressionReason)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate time-series statistics: %w", err)
	}
	return points, nil
}

func applyPublicationPolicy(sampleSize int64, publishable *bool, reason **string) {
	*publishable = sampleSize >= descriptivePublicationThreshold
	if *publishable {
		*reason = nil
		return
	}
	value := "below_minimum_sample_30"
	*reason = &value
}

func (r *StatisticsRepository) Outliers(
	ctx context.Context,
	filter StatisticsFilter,
	limit, offset int,
) ([]OutlierRecord, int64, error) {
	geographyClause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`
		SELECT
			count(*) OVER ()::bigint,
			fact.transaction_id,
			fact.contract_date::text,
			fact.property_group,
			fact.area::float8,
			fact.price::float8,
			fact.price_per_sqm::float8,
			fact.cadastral_municipality_code,
			fact.building_number,
			fact.building_part_number,
			evaluation.cohort_level,
			evaluation.cohort_size,
			evaluation.cohort_median::float8,
			evaluation.lower_bound::float8,
			evaluation.upper_bound::float8,
			evaluation.modified_z_score::float8,
			evaluation.rule_version
		FROM %s fact
		JOIN analytics.sale_outlier_evaluations evaluation USING (transaction_id)
		WHERE evaluation.status='outlier'
		  AND ($1::date IS NULL OR fact.contract_date >= $1)
		  AND ($2::date IS NULL OR fact.contract_date <= $2)
		  AND ($3 = '' OR fact.property_group = $3)
		  AND %s
		ORDER BY evaluation.modified_z_score DESC NULLS LAST,fact.transaction_id
		LIMIT $7 OFFSET $8`, saleFactsTable(filter), geographyClause)
	rows, err := r.pool.Query(ctx, query,
		filter.FromDate, filter.ToDate, filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list statistical outliers: %w", err)
	}
	defer rows.Close()
	items := make([]OutlierRecord, 0)
	var total int64
	for rows.Next() {
		var item OutlierRecord
		if err := rows.Scan(
			&total, &item.TransactionID, &item.ContractDate, &item.PropertyGroup,
			&item.Area, &item.Price, &item.PricePerSQM,
			&item.CadastralMunicipalityCode, &item.BuildingNumber, &item.BuildingPartNumber,
			&item.CohortLevel, &item.CohortSize, &item.CohortMedian,
			&item.LowerBound, &item.UpperBound, &item.ModifiedZScore, &item.RuleVersion,
		); err != nil {
			return nil, 0, fmt.Errorf("scan statistical outlier: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate statistical outliers: %w", err)
	}
	return items, total, nil
}

func (r *StatisticsRepository) geographyClause(
	ctx context.Context,
	geographyID *int64,
) (string, *int64, error) {
	if geographyID == nil {
		return "$4::bigint IS NULL AND ($5::float8 IS NULL OR area >= $5) AND ($6::float8 IS NULL OR area <= $6)", nil, nil
	}
	var level string
	err := r.pool.QueryRow(ctx,
		"SELECT level FROM analytics.geographic_units WHERE id=$1", *geographyID,
	).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrGeographyNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("resolve statistics geography: %w", err)
	}
	column, ok := geographyFactColumn(level)
	if !ok {
		return "", nil, fmt.Errorf("unsupported geography level %q", level)
	}
	if column == "" {
		return "$4::bigint IS NOT NULL AND ($5::float8 IS NULL OR area >= $5) AND ($6::float8 IS NULL OR area <= $6)", geographyID, nil
	}
	return column + " = $4 AND ($5::float8 IS NULL OR area >= $5) AND ($6::float8 IS NULL OR area <= $6)", geographyID, nil
}

func geographyFactColumn(level string) (string, bool) {
	columns := map[string]string{
		"country":            "",
		"statistical_region": "statistical_region_id",
		"municipality":       "municipality_id",
		"city_district":      "city_district_id",
		"local_community":    "local_community_id",
		"settlement":         "settlement_id",
	}
	column, ok := columns[level]
	return column, ok
}

func intervalSQL(interval string) (string, string) {
	switch interval {
	case "month":
		return "month", "1 month"
	case "quarter":
		return "quarter", "3 months"
	default:
		return "year", "1 year"
	}
}
