CREATE TABLE IF NOT EXISTS analytics.rental_outlier_evaluations (
    transaction_id bigint PRIMARY KEY
        REFERENCES analytics.rental_facts(transaction_id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN (
        'inlier','outlier','insufficient_cohort','ineligible'
    )),
    method text,
    cohort_level text,
    cohort_size integer,
    cohort_median numeric(18,4),
    lower_bound numeric(18,4),
    upper_bound numeric(18,4),
    modified_z_score numeric(18,6),
    rule_version text NOT NULL,
    reasons text[] NOT NULL DEFAULT ARRAY[]::text[],
    evaluated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS rental_outlier_status_idx
    ON analytics.rental_outlier_evaluations(status);

CREATE OR REPLACE FUNCTION analytics.refresh_rental_outliers()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    evaluated_rows bigint;
    refresh_run_id bigint;
BEGIN
    INSERT INTO analytics.refresh_runs(process,status)
    VALUES ('rental_outliers','running') RETURNING id INTO refresh_run_id;

    DROP TABLE IF EXISTS pg_temp.rental_outlier_base;
    CREATE TEMP TABLE rental_outlier_base ON COMMIT DROP AS
    SELECT
        transaction_id,property_group,monthly_rent_per_sqm,municipality_id,
        statistical_region_id,floor(area/20.0)*20 AS area_band,
        extract(year FROM contract_date)::integer AS contract_year
    FROM analytics.rental_facts
    WHERE is_quality_eligible AND monthly_rent_per_sqm>0;
    CREATE INDEX ON rental_outlier_base
        (municipality_id,property_group,area_band,contract_year);
    CREATE INDEX ON rental_outlier_base
        (statistical_region_id,property_group,area_band,contract_year);

    DROP TABLE IF EXISTS pg_temp.rental_outlier_municipality;
    CREATE TEMP TABLE rental_outlier_municipality ON COMMIT DROP AS
    SELECT
        municipality_id,property_group,area_band,contract_year,
        count(*)::integer AS cohort_size,
        percentile_cont(0.25) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS q1,
        percentile_cont(0.5) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS median,
        percentile_cont(0.75) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS q3
    FROM rental_outlier_base
    WHERE municipality_id IS NOT NULL
    GROUP BY 1,2,3,4;

    DROP TABLE IF EXISTS pg_temp.rental_outlier_region;
    CREATE TEMP TABLE rental_outlier_region ON COMMIT DROP AS
    SELECT
        statistical_region_id,property_group,area_band,contract_year,
        count(*)::integer AS cohort_size,
        percentile_cont(0.25) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS q1,
        percentile_cont(0.5) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS median,
        percentile_cont(0.75) WITHIN GROUP(ORDER BY monthly_rent_per_sqm) AS q3
    FROM rental_outlier_base
    WHERE statistical_region_id IS NOT NULL
    GROUP BY 1,2,3,4;

    TRUNCATE analytics.rental_outlier_evaluations;
    INSERT INTO analytics.rental_outlier_evaluations(
        transaction_id,status,method,cohort_level,cohort_size,cohort_median,
        lower_bound,upper_bound,modified_z_score,rule_version,reasons
    )
    SELECT
        fact.transaction_id,
        CASE
            WHEN NOT fact.is_quality_eligible THEN 'ineligible'
            WHEN chosen.cohort_size IS NULL THEN 'insufficient_cohort'
            WHEN analytics.is_statistical_outlier(
                fact.monthly_rent_per_sqm,chosen.q1::numeric,chosen.q3::numeric,
                chosen.median::numeric,
                ((chosen.q3-chosen.q1)/1.349)::numeric
            ) THEN 'outlier'
            ELSE 'inlier'
        END,
        CASE WHEN chosen.cohort_size IS NOT NULL THEN 'iqr_and_approximate_mad' END,
        chosen.cohort_level,chosen.cohort_size,chosen.median,
        chosen.q1-1.5*greatest(chosen.q3-chosen.q1,abs(chosen.median)*0.20),
        chosen.q3+1.5*greatest(chosen.q3-chosen.q1,abs(chosen.median)*0.20),
        CASE WHEN chosen.median>0 THEN
            0.6745*abs(fact.monthly_rent_per_sqm-chosen.median)
            /greatest((chosen.q3-chosen.q1)/1.349,abs(chosen.median)*0.10)
        END,
        'rental-outlier-v1',
        CASE
            WHEN NOT fact.is_quality_eligible THEN fact.quality_reasons
            WHEN chosen.cohort_size IS NULL
                THEN ARRAY['insufficient_comparable_cohort']::text[]
            WHEN analytics.is_statistical_outlier(
                fact.monthly_rent_per_sqm,chosen.q1::numeric,chosen.q3::numeric,
                chosen.median::numeric,
                ((chosen.q3-chosen.q1)/1.349)::numeric
            ) THEN ARRAY['statistically_atypical_monthly_rent_per_sqm']::text[]
            ELSE ARRAY[]::text[]
        END
    FROM analytics.rental_facts fact
    LEFT JOIN rental_outlier_base base USING(transaction_id)
    LEFT JOIN rental_outlier_municipality municipality
      ON municipality.municipality_id=base.municipality_id
     AND municipality.property_group=base.property_group
     AND municipality.area_band=base.area_band
     AND municipality.contract_year=base.contract_year
    LEFT JOIN rental_outlier_region region
      ON region.statistical_region_id=base.statistical_region_id
     AND region.property_group=base.property_group
     AND region.area_band=base.area_band
     AND region.contract_year=base.contract_year
    LEFT JOIN LATERAL (
        SELECT *
        FROM (
            SELECT 'municipality_year'::text AS cohort_level,
                   municipality.cohort_size,municipality.q1,
                   municipality.median,municipality.q3,1 AS priority
            WHERE municipality.cohort_size>=15
            UNION ALL
            SELECT 'statistical_region_year',region.cohort_size,region.q1,
                   region.median,region.q3,2
            WHERE region.cohort_size>=30
        ) cohorts ORDER BY priority LIMIT 1
    ) chosen ON true;

    UPDATE analytics.rental_facts fact
    SET representative_eligible=(
        fact.is_quality_eligible AND evaluation.status<>'outlier'
    )
    FROM analytics.rental_outlier_evaluations evaluation
    WHERE evaluation.transaction_id=fact.transaction_id;

    GET DIAGNOSTICS evaluated_rows=ROW_COUNT;
    ANALYZE analytics.rental_facts;
    ANALYZE analytics.rental_outlier_evaluations;
    UPDATE analytics.refresh_runs
    SET status='succeeded',finished_at=clock_timestamp(),
        details=jsonb_build_object(
            'rows',evaluated_rows,'ruleVersion','rental-outlier-v1'
        )
    WHERE id=refresh_run_id;
    RETURN evaluated_rows;
END
$$;

COMMENT ON TABLE analytics.rental_outlier_evaluations IS
    'Auditable robust monthly-rent/m2 cohort evaluations; source rentals are never deleted.';
