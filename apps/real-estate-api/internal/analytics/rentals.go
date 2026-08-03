package analytics

import (
	"context"
	"fmt"
	"time"
)

const rentalWeakSampleThreshold = 20

type RentalOverview struct {
	TransactionCount        int64    `json:"transactionCount"`
	EligibleCount           int64    `json:"eligibleCount"`
	IncludedSampleSize      int64    `json:"includedSampleSize"`
	ExcludedOutlierCount    int64    `json:"excludedOutlierCount"`
	MonthlyRentSampleSize   int64    `json:"monthlyRentSampleSize"`
	RentPerSQMSampleSize    int64    `json:"rentPerSqmSampleSize"`
	MedianMonthlyRent       *float64 `json:"medianMonthlyRent"`
	CleanedAverageRent      *float64 `json:"cleanedAverageMonthlyRent"`
	RawAverageRent          *float64 `json:"rawAverageMonthlyRent"`
	MedianMonthlyRentPerSQM *float64 `json:"medianMonthlyRentPerSqm"`
	CleanedAveragePerSQM    *float64 `json:"cleanedAverageMonthlyRentPerSqm"`
	RawAveragePerSQM        *float64 `json:"rawAverageMonthlyRentPerSqm"`
	MedianArea              *float64 `json:"medianArea"`
	IsPublishable           bool     `json:"isPublishable"`
	SuppressionReason       *string  `json:"suppressionReason"`
}

type RentalTimeSeriesPoint struct {
	Period                   string   `json:"period"`
	Incomplete               bool     `json:"incomplete"`
	TransactionCount         int64    `json:"transactionCount"`
	IncludedSampleSize       int64    `json:"includedSampleSize"`
	MedianMonthlyRent        *float64 `json:"medianMonthlyRent"`
	MedianMonthlyRentPerSQM  *float64 `json:"medianMonthlyRentPerSqm"`
	PreviousYearSampleSize   *int64   `json:"previousYearSampleSize"`
	PreviousYearMedianRent   *float64 `json:"previousYearMedianMonthlyRent"`
	PreviousYearMedianPerSQM *float64 `json:"previousYearMedianMonthlyRentPerSqm"`
	MonthlyRentYoYPercent    *float64 `json:"monthlyRentYoYPercent"`
	RentPerSQMYoYPercent     *float64 `json:"rentPerSqmYoYPercent"`
	WeakComparisonSample     bool     `json:"weakComparisonSample"`
	IsPublishable            bool     `json:"isPublishable"`
	SuppressionReason        *string  `json:"suppressionReason"`
}

type RentalOutlierRecord struct {
	TransactionID     int64    `json:"transactionId"`
	ContractDate      *string  `json:"contractDate"`
	PropertyGroup     string   `json:"propertyGroup"`
	Area              *float64 `json:"area"`
	MonthlyRent       *float64 `json:"monthlyRent"`
	MonthlyRentPerSQM *float64 `json:"monthlyRentPerSqm"`
	CohortLevel       string   `json:"cohortLevel"`
	CohortSize        int      `json:"cohortSize"`
	CohortMedian      float64  `json:"cohortMedian"`
	LowerBound        float64  `json:"lowerBound"`
	UpperBound        float64  `json:"upperBound"`
	ModifiedZScore    *float64 `json:"modifiedZScore"`
	RuleVersion       string   `json:"ruleVersion"`
}

type RentalPropertyPoint struct {
	TransactionID             int64    `json:"transactionId"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode"`
	BuildingNumber            *string  `json:"buildingNumber"`
	Longitude                 float64  `json:"longitude"`
	Latitude                  float64  `json:"latitude"`
	ContractDate              *string  `json:"contractDate"`
	PropertyGroup             string   `json:"propertyGroup"`
	Area                      *float64 `json:"area"`
	MonthlyRent               *float64 `json:"monthlyRent"`
	MonthlyRentPerSQM         *float64 `json:"monthlyRentPerSqm"`
	IsRepresentative          bool     `json:"isRepresentative"`
}

type RentalStatisticsStore interface {
	RentalOverview(context.Context, StatisticsFilter) (RentalOverview, error)
	RentalTimeSeries(context.Context, StatisticsFilter, string) ([]RentalTimeSeriesPoint, error)
	RentalOutliers(context.Context, StatisticsFilter, int, int) ([]RentalOutlierRecord, int64, error)
	RentalClusters(context.Context, StatisticsFilter, Bounds, float64) ([]Cluster, error)
	RentalPropertyPoints(context.Context, StatisticsFilter, Bounds, int) ([]RentalPropertyPoint, error)
}

func (r *StatisticsRepository) RentalClusters(
	ctx context.Context, filter StatisticsFilter, bounds Bounds, gridSize float64,
) ([]Cluster, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		WITH bounded AS (
		  SELECT ST_SnapToGrid(ST_Transform(location,4326),$11,$11) grid_point,
		    monthly_rent_per_sqm,monthly_rent_ex_vat,representative_eligible
		  FROM analytics.rental_facts
		  WHERE location IS NOT NULL
		    AND location && ST_Transform(ST_MakeEnvelope($7,$8,$9,$10,4326),3794)
		    AND ($1::date IS NULL OR contract_date >= $1)
		    AND ($2::date IS NULL OR contract_date <= $2)
		    AND ($3 = '' OR property_group=$3) AND %s
		)
		SELECT ST_X(grid_point),ST_Y(grid_point),count(*)::bigint,
		  count(*) FILTER (WHERE representative_eligible)::bigint,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_per_sqm)
		    FILTER (WHERE representative_eligible)::float8,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_ex_vat)
		    FILTER (WHERE representative_eligible)::float8
		FROM bounded GROUP BY grid_point ORDER BY count(*) DESC LIMIT 5000`, clause)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea, bounds.West, bounds.South,
		bounds.East, bounds.North, gridSize)
	if err != nil {
		return nil, fmt.Errorf("rental map clusters: %w", err)
	}
	defer rows.Close()
	items := make([]Cluster, 0)
	for rows.Next() {
		var item Cluster
		if err := rows.Scan(&item.Longitude, &item.Latitude,
			&item.TransactionCount, &item.RepresentativeCount,
			&item.MedianPricePerSQM, &item.MedianAmount); err != nil {
			return nil, fmt.Errorf("scan rental cluster: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StatisticsRepository) RentalPropertyPoints(
	ctx context.Context, filter StatisticsFilter, bounds Bounds, limit int,
) ([]RentalPropertyPoint, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT transaction_id,cadastral_municipality_code,building_number,
		  ST_X(ST_Transform(location,4326)),
		  ST_Y(ST_Transform(location,4326)),contract_date::text,property_group,
		  area::float8,monthly_rent_ex_vat::float8,
		  monthly_rent_per_sqm::float8,representative_eligible
		FROM analytics.rental_facts
		WHERE location IS NOT NULL
		  AND location && ST_Transform(ST_MakeEnvelope($7,$8,$9,$10,4326),3794)
		  AND ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group=$3) AND %s
		ORDER BY contract_date DESC NULLS LAST,transaction_id LIMIT $11`, clause)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea, bounds.West, bounds.South,
		bounds.East, bounds.North, limit)
	if err != nil {
		return nil, fmt.Errorf("rental map points: %w", err)
	}
	defer rows.Close()
	items := make([]RentalPropertyPoint, 0)
	for rows.Next() {
		var item RentalPropertyPoint
		if err := rows.Scan(&item.TransactionID, &item.CadastralMunicipalityCode,
			&item.BuildingNumber, &item.Longitude, &item.Latitude,
			&item.ContractDate, &item.PropertyGroup, &item.Area, &item.MonthlyRent,
			&item.MonthlyRentPerSQM, &item.IsRepresentative); err != nil {
			return nil, fmt.Errorf("scan rental map point: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StatisticsRepository) RentalOverview(
	ctx context.Context, filter StatisticsFilter,
) (RentalOverview, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return RentalOverview{}, err
	}
	query := fmt.Sprintf(`
		SELECT count(*)::bigint,
		  count(*) FILTER (WHERE is_quality_eligible)::bigint,
		  count(*) FILTER (WHERE representative_eligible)::bigint,
		  count(*) FILTER (WHERE is_quality_eligible AND NOT representative_eligible)::bigint,
		  count(monthly_rent_ex_vat) FILTER (WHERE representative_eligible)::bigint,
		  count(monthly_rent_per_sqm) FILTER (WHERE representative_eligible)::bigint,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_ex_vat)
		    FILTER (WHERE representative_eligible)::float8,
		  avg(monthly_rent_ex_vat) FILTER (WHERE representative_eligible)::float8,
		  avg(monthly_rent_ex_vat) FILTER (WHERE is_quality_eligible)::float8,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_per_sqm)
		    FILTER (WHERE representative_eligible)::float8,
		  avg(monthly_rent_per_sqm) FILTER (WHERE representative_eligible)::float8,
		  avg(monthly_rent_per_sqm) FILTER (WHERE is_quality_eligible)::float8,
		  percentile_cont(.5) WITHIN GROUP (ORDER BY area)
		    FILTER (WHERE representative_eligible)::float8
		FROM analytics.rental_facts
		WHERE ($1::date IS NULL OR contract_date >= $1)
		  AND ($2::date IS NULL OR contract_date <= $2)
		  AND ($3 = '' OR property_group=$3) AND %s`, clause)
	var result RentalOverview
	err = r.pool.QueryRow(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea).Scan(
		&result.TransactionCount, &result.EligibleCount, &result.IncludedSampleSize,
		&result.ExcludedOutlierCount, &result.MonthlyRentSampleSize,
		&result.RentPerSQMSampleSize, &result.MedianMonthlyRent,
		&result.CleanedAverageRent, &result.RawAverageRent,
		&result.MedianMonthlyRentPerSQM, &result.CleanedAveragePerSQM,
		&result.RawAveragePerSQM, &result.MedianArea)
	if err != nil {
		return RentalOverview{}, fmt.Errorf("rental overview statistics: %w", err)
	}
	applyPublicationPolicy(result.IncludedSampleSize,
		&result.IsPublishable, &result.SuppressionReason)
	return result, nil
}

func (r *StatisticsRepository) RentalTimeSeries(
	ctx context.Context, filter StatisticsFilter, interval string,
) ([]RentalTimeSeriesPoint, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	unit, step := intervalSQL(interval)
	lag := map[string]int{"month": 12, "quarter": 4, "year": 1}[interval]
	query := fmt.Sprintf(`
		WITH periods AS (
		  SELECT date_trunc('%s',contract_date)::date period,
		    (date_trunc('%s',contract_date)+interval '%s'>current_date) incomplete,
		    count(*)::bigint transaction_count,
		    count(*) FILTER (WHERE representative_eligible)::bigint sample_size,
		    percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_ex_vat)
		      FILTER (WHERE representative_eligible)::float8 median_rent,
		    percentile_cont(.5) WITHIN GROUP (ORDER BY monthly_rent_per_sqm)
		      FILTER (WHERE representative_eligible)::float8 median_per_sqm
		  FROM analytics.rental_facts
		  WHERE contract_date IS NOT NULL
		    AND ($1::date IS NULL OR contract_date >= $1)
		    AND ($2::date IS NULL OR contract_date <= $2)
		    AND ($3 = '' OR property_group=$3) AND %s
		  GROUP BY 1,2
		), compared AS (
		  SELECT *,
		    lag(sample_size,%d) OVER (ORDER BY period) previous_size,
		    lag(median_rent,%d) OVER (ORDER BY period) previous_rent,
		    lag(median_per_sqm,%d) OVER (ORDER BY period) previous_per_sqm,
		    lag(period,%d) OVER (ORDER BY period) previous_period
		  FROM periods
		)
		SELECT period,incomplete,transaction_count,sample_size,median_rent,
		  median_per_sqm,
		  CASE WHEN previous_period=period-interval '1 year' THEN previous_size END,
		  CASE WHEN previous_period=period-interval '1 year' THEN previous_rent END,
		  CASE WHEN previous_period=period-interval '1 year' THEN previous_per_sqm END,
		  CASE WHEN previous_period=period-interval '1 year' AND previous_rent>0
		    THEN 100*(median_rent/previous_rent-1) END,
		  CASE WHEN previous_period=period-interval '1 year' AND previous_per_sqm>0
		    THEN 100*(median_per_sqm/previous_per_sqm-1) END
		FROM compared ORDER BY period`,
		unit, unit, step, clause, lag, lag, lag, lag)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea)
	if err != nil {
		return nil, fmt.Errorf("rental time-series statistics: %w", err)
	}
	defer rows.Close()
	items := make([]RentalTimeSeriesPoint, 0)
	for rows.Next() {
		var period time.Time
		var item RentalTimeSeriesPoint
		if err := rows.Scan(&period, &item.Incomplete, &item.TransactionCount,
			&item.IncludedSampleSize, &item.MedianMonthlyRent,
			&item.MedianMonthlyRentPerSQM, &item.PreviousYearSampleSize,
			&item.PreviousYearMedianRent, &item.PreviousYearMedianPerSQM,
			&item.MonthlyRentYoYPercent, &item.RentPerSQMYoYPercent); err != nil {
			return nil, fmt.Errorf("scan rental time-series: %w", err)
		}
		item.Period = period.Format("2006-01-02")
		item.WeakComparisonSample = item.IncludedSampleSize < rentalWeakSampleThreshold ||
			item.PreviousYearSampleSize == nil ||
			*item.PreviousYearSampleSize < rentalWeakSampleThreshold
		applyPublicationPolicy(item.IncludedSampleSize,
			&item.IsPublishable, &item.SuppressionReason)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StatisticsRepository) RentalOutliers(
	ctx context.Context, filter StatisticsFilter, limit, offset int,
) ([]RentalOutlierRecord, int64, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`
		SELECT count(*) OVER ()::bigint,f.transaction_id,f.contract_date::text,
		  f.property_group,f.area::float8,f.monthly_rent_ex_vat::float8,
		  f.monthly_rent_per_sqm::float8,e.cohort_level,e.cohort_size,
		  e.cohort_median::float8,e.lower_bound::float8,e.upper_bound::float8,
		  e.modified_z_score::float8,e.rule_version
		FROM analytics.rental_facts f
		JOIN analytics.rental_outlier_evaluations e USING(transaction_id)
		WHERE e.status='outlier'
		  AND ($1::date IS NULL OR f.contract_date >= $1)
		  AND ($2::date IS NULL OR f.contract_date <= $2)
		  AND ($3 = '' OR f.property_group=$3) AND %s
		ORDER BY e.modified_z_score DESC NULLS LAST,f.transaction_id
		LIMIT $7 OFFSET $8`, clause)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list rental outliers: %w", err)
	}
	defer rows.Close()
	var total int64
	items := make([]RentalOutlierRecord, 0)
	for rows.Next() {
		var item RentalOutlierRecord
		if err := rows.Scan(&total, &item.TransactionID, &item.ContractDate,
			&item.PropertyGroup, &item.Area, &item.MonthlyRent,
			&item.MonthlyRentPerSQM, &item.CohortLevel, &item.CohortSize,
			&item.CohortMedian, &item.LowerBound, &item.UpperBound,
			&item.ModifiedZScore, &item.RuleVersion); err != nil {
			return nil, 0, fmt.Errorf("scan rental outlier: %w", err)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
