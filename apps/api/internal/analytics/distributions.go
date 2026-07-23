package analytics

import (
	"context"
	"fmt"
)

type Percentiles struct {
	P05 float64 `json:"p05"`
	P10 float64 `json:"p10"`
	P25 float64 `json:"p25"`
	P50 float64 `json:"p50"`
	P75 float64 `json:"p75"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
}

type HistogramBucket struct {
	Index int     `json:"index"`
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Count int64   `json:"count"`
}

type Distribution struct {
	Metric              string            `json:"metric"`
	SampleSize          int64             `json:"sampleSize"`
	Percentiles         *Percentiles      `json:"percentiles"`
	HistogramLowerBound *float64          `json:"histogramLowerBound"`
	HistogramUpperBound *float64          `json:"histogramUpperBound"`
	BelowBoundCount     int64             `json:"belowBoundCount"`
	AboveBoundCount     int64             `json:"aboveBoundCount"`
	Buckets             []HistogramBucket `json:"buckets"`
}

type DistributionStore interface {
	SaleDistribution(context.Context, StatisticsFilter, string, int) (Distribution, error)
	RentalDistribution(context.Context, StatisticsFilter, string, int) (Distribution, error)
}

func (r *StatisticsRepository) SaleDistribution(
	ctx context.Context, filter StatisticsFilter, metric string, bins int,
) (Distribution, error) {
	column := map[string]string{"price": "price", "pricePerSqm": "price_per_sqm"}[metric]
	return r.distribution(ctx, filter, "analytics.sale_facts", column, metric, bins)
}

func (r *StatisticsRepository) RentalDistribution(
	ctx context.Context, filter StatisticsFilter, metric string, bins int,
) (Distribution, error) {
	column := map[string]string{
		"monthlyRent": "monthly_rent_ex_vat",
		"rentPerSqm":  "monthly_rent_per_sqm",
	}[metric]
	return r.distribution(ctx, filter, "analytics.rental_facts", column, metric, bins)
}

func (r *StatisticsRepository) distribution(
	ctx context.Context, filter StatisticsFilter, table, column, metric string, bins int,
) (Distribution, error) {
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return Distribution{}, err
	}
	query := fmt.Sprintf(`
		WITH values AS MATERIALIZED (
		  SELECT %s::float8 value FROM %s
		  WHERE representative_eligible AND %s IS NOT NULL
		    AND ($1::date IS NULL OR contract_date >= $1)
		    AND ($2::date IS NULL OR contract_date <= $2)
		    AND ($3 = '' OR property_group=$3) AND %s
		), summary AS (
		  SELECT count(*)::bigint sample_size,
		    percentile_cont(.05) WITHIN GROUP(ORDER BY value) p05,
		    percentile_cont(.10) WITHIN GROUP(ORDER BY value) p10,
		    percentile_cont(.25) WITHIN GROUP(ORDER BY value) p25,
		    percentile_cont(.50) WITHIN GROUP(ORDER BY value) p50,
		    percentile_cont(.75) WITHIN GROUP(ORDER BY value) p75,
		    percentile_cont(.90) WITHIN GROUP(ORDER BY value) p90,
		    percentile_cont(.95) WITHIN GROUP(ORDER BY value) p95,
		    percentile_cont(.01) WITHIN GROUP(ORDER BY value) low,
		    percentile_cont(.99) WITHIN GROUP(ORDER BY value) high
		  FROM values
		), bucketed AS (
		  SELECT greatest(1,least($5,width_bucket(value,s.low,s.high,$5))) bucket,
		    count(*)::bigint count
		  FROM values CROSS JOIN summary s
		  WHERE s.high>s.low GROUP BY 1
		), tails AS (
		  SELECT count(*) FILTER(WHERE v.value<s.low)::bigint below_count,
		    count(*) FILTER(WHERE v.value>s.high)::bigint above_count
		  FROM values v CROSS JOIN summary s
		)
		SELECT s.sample_size,s.p05,s.p10,s.p25,s.p50,s.p75,s.p90,s.p95,
		  s.low,s.high,
		  t.below_count,t.above_count,
		  series.bucket,
		  s.low+(series.bucket-1)*(s.high-s.low)/$5 bucket_from,
		  s.low+series.bucket*(s.high-s.low)/$5 bucket_to,
		  coalesce(b.count,0)
		FROM summary s
		CROSS JOIN tails t
		CROSS JOIN generate_series(1,$5) series(bucket)
		LEFT JOIN bucketed b ON b.bucket=series.bucket
		GROUP BY s.sample_size,s.p05,s.p10,s.p25,s.p50,s.p75,s.p90,s.p95,
		  s.low,s.high,t.below_count,t.above_count,series.bucket,b.count
		ORDER BY series.bucket`, column, table, column, clause)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, bins)
	if err != nil {
		return Distribution{}, fmt.Errorf("calculate %s distribution: %w", metric, err)
	}
	defer rows.Close()
	result := Distribution{Metric: metric, Buckets: make([]HistogramBucket, 0, bins)}
	for rows.Next() {
		var p05, p10, p25, p50, p75, p90, p95 *float64
		var low, high *float64
		var bucketFrom, bucketTo *float64
		var bucket HistogramBucket
		if err := rows.Scan(&result.SampleSize, &p05, &p10, &p25, &p50, &p75,
			&p90, &p95, &low, &high,
			&result.BelowBoundCount, &result.AboveBoundCount, &bucket.Index,
			&bucketFrom, &bucketTo, &bucket.Count); err != nil {
			return Distribution{}, fmt.Errorf("scan distribution: %w", err)
		}
		result.HistogramLowerBound, result.HistogramUpperBound = low, high
		if p50 != nil {
			result.Percentiles = &Percentiles{
				P05: *p05, P10: *p10, P25: *p25, P50: *p50,
				P75: *p75, P90: *p90, P95: *p95,
			}
		}
		if bucketFrom != nil && bucketTo != nil {
			bucket.From, bucket.To = *bucketFrom, *bucketTo
			result.Buckets = append(result.Buckets, bucket)
		}
	}
	if err := rows.Err(); err != nil {
		return Distribution{}, fmt.Errorf("iterate distribution: %w", err)
	}
	if result.SampleSize == 0 {
		result.Percentiles = nil
		result.Buckets = []HistogramBucket{}
	}
	return result, nil
}
