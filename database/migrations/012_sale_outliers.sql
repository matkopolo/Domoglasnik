ALTER TABLE analytics.sale_facts
    ADD COLUMN IF NOT EXISTS cadastral_municipality_code integer,
    ADD COLUMN IF NOT EXISTS building_number text,
    ADD COLUMN IF NOT EXISTS building_part_number text,
    ADD COLUMN IF NOT EXISTS construction_year integer,
    ADD COLUMN IF NOT EXISTS representative_eligible boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS sale_facts_building_idx
    ON analytics.sale_facts (cadastral_municipality_code,building_number,property_group)
    WHERE cadastral_municipality_code IS NOT NULL AND building_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS sale_facts_representative_date_idx
    ON analytics.sale_facts (representative_eligible,contract_date);

CREATE TABLE IF NOT EXISTS analytics.sale_outlier_evaluations (
    transaction_id bigint PRIMARY KEY
        REFERENCES analytics.sale_facts(transaction_id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN (
        'inlier', 'outlier', 'insufficient_cohort', 'ineligible'
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

CREATE INDEX IF NOT EXISTS sale_outlier_status_idx
    ON analytics.sale_outlier_evaluations (status);

CREATE OR REPLACE FUNCTION analytics.is_statistical_outlier(
    observed_value numeric,
    first_quartile numeric,
    third_quartile numeric,
    cohort_median numeric,
    median_absolute_deviation numeric
) RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
SELECT CASE
    WHEN observed_value IS NULL THEN false
    ELSE (
        (cohort_median>0 AND (
            observed_value<first_quartile-1.5*greatest(
                third_quartile-first_quartile,abs(cohort_median)*0.20
            )
            OR observed_value>third_quartile+1.5*greatest(
                third_quartile-first_quartile,abs(cohort_median)*0.20
            )
        ))
        OR
        (cohort_median>0 AND 0.6745*abs(observed_value-cohort_median)
         /greatest(median_absolute_deviation,abs(cohort_median)*0.10)>3.5)
    )
END
$$;

CREATE OR REPLACE FUNCTION analytics.refresh_sale_outliers()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    evaluated_rows bigint;
    refresh_run_id bigint;
BEGIN
    INSERT INTO analytics.refresh_runs(process,status)
    VALUES ('sale_outliers','running') RETURNING id INTO refresh_run_id;

    WITH attributes AS (
        SELECT
            bp.transaction_id,
            min(bp.cadastral_municipality_code) AS cadastral_municipality_code,
            min(bp.building_number) AS building_number,
            min(bp.building_part_number) AS building_part_number,
            min(bp.construction_year) AS construction_year
        FROM building_parts bp
        JOIN analytics.sale_facts fact ON fact.transaction_id=bp.transaction_id
        LEFT JOIN analytics.building_part_type_groups mapping
          ON mapping.source_code=bp.building_part_type
        WHERE (mapping.is_primary AND mapping.property_group=fact.property_group)
           OR (fact.property_group='garage' AND mapping.property_group='garage')
        GROUP BY bp.transaction_id
    )
    UPDATE analytics.sale_facts fact
    SET cadastral_municipality_code=attributes.cadastral_municipality_code,
        building_number=attributes.building_number,
        building_part_number=attributes.building_part_number,
        construction_year=attributes.construction_year
    FROM attributes
    WHERE attributes.transaction_id=fact.transaction_id;

    UPDATE analytics.sale_facts
    SET is_quality_eligible=false,
        quality_reasons=array_append(quality_reasons,'implausible_area_for_property_group')
    WHERE is_quality_eligible
      AND (
        (property_group='apartment' AND (area<10 OR area>1000))
        OR (property_group='house' AND (area<20 OR area>2000))
        OR (property_group='commercial' AND (area<5 OR area>50000))
        OR (property_group='garage' AND (area<2 OR area>200))
        OR (property_group='land' AND area<1)
      )
      AND NOT quality_reasons@>ARRAY['implausible_area_for_property_group']::text[];

    UPDATE analytics.sale_facts
    SET is_quality_eligible=false,
        quality_reasons=array_append(quality_reasons,'implausible_price_per_sqm')
    WHERE is_quality_eligible
      AND (
        (property_group='apartment' AND (price_per_sqm<100 OR price_per_sqm>50000))
        OR (property_group='house' AND (price_per_sqm<20 OR price_per_sqm>50000))
        OR (property_group='commercial' AND (price_per_sqm<10 OR price_per_sqm>100000))
        OR (property_group='garage' AND (price_per_sqm<20 OR price_per_sqm>50000))
      )
      AND NOT quality_reasons@>ARRAY['implausible_price_per_sqm']::text[];

    DROP TABLE IF EXISTS pg_temp.sale_outlier_base;
    CREATE TEMP TABLE sale_outlier_base ON COMMIT DROP AS
    SELECT
        transaction_id,property_group,price_per_sqm,municipality_id,
        statistical_region_id,cadastral_municipality_code,building_number,
        floor(area/20.0)*20 AS area_band,
        extract(year FROM contract_date)::integer AS contract_year
    FROM analytics.sale_facts
    WHERE is_quality_eligible AND price_per_sqm>0
      AND property_group IN ('apartment','house','commercial','garage');
    CREATE INDEX ON sale_outlier_base
        (cadastral_municipality_code,building_number,property_group,area_band,contract_year);
    CREATE INDEX ON sale_outlier_base
        (municipality_id,property_group,area_band,contract_year);
    CREATE INDEX ON sale_outlier_base
        (statistical_region_id,property_group,area_band,contract_year);

    DROP TABLE IF EXISTS pg_temp.sale_outlier_building;
    CREATE TEMP TABLE sale_outlier_building ON COMMIT DROP AS
    SELECT
        cadastral_municipality_code,building_number,property_group,area_band,contract_year,
        count(*)::integer AS cohort_size,
        percentile_cont(0.25) WITHIN GROUP (ORDER BY price_per_sqm) AS q1,
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm) AS median,
        percentile_cont(0.75) WITHIN GROUP (ORDER BY price_per_sqm) AS q3,
        NULL::numeric AS mad
    FROM sale_outlier_base
    WHERE cadastral_municipality_code IS NOT NULL AND building_number IS NOT NULL
    GROUP BY 1,2,3,4,5;
    UPDATE sale_outlier_building summary
    SET mad=deviation.mad
    FROM (
        SELECT
            base.cadastral_municipality_code,base.building_number,
            base.property_group,base.area_band,base.contract_year,
            percentile_cont(0.5) WITHIN GROUP (
                ORDER BY abs(base.price_per_sqm-summary_inner.median)
            ) AS mad
        FROM sale_outlier_base base
        JOIN sale_outlier_building summary_inner
          USING (cadastral_municipality_code,building_number,property_group,area_band,contract_year)
        GROUP BY 1,2,3,4,5
    ) deviation
    WHERE summary.cadastral_municipality_code=deviation.cadastral_municipality_code
      AND summary.building_number=deviation.building_number
      AND summary.property_group=deviation.property_group
      AND summary.area_band=deviation.area_band
      AND summary.contract_year=deviation.contract_year;

    DROP TABLE IF EXISTS pg_temp.sale_outlier_municipality;
    CREATE TEMP TABLE sale_outlier_municipality ON COMMIT DROP AS
    SELECT
        municipality_id,property_group,area_band,contract_year,
        count(*)::integer AS cohort_size,
        percentile_cont(0.25) WITHIN GROUP (ORDER BY price_per_sqm) AS q1,
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm) AS median,
        percentile_cont(0.75) WITHIN GROUP (ORDER BY price_per_sqm) AS q3,
        NULL::numeric AS mad
    FROM sale_outlier_base
    WHERE municipality_id IS NOT NULL
    GROUP BY 1,2,3,4;
    UPDATE sale_outlier_municipality summary
    SET mad=deviation.mad
    FROM (
        SELECT
            base.municipality_id,base.property_group,base.area_band,base.contract_year,
            percentile_cont(0.5) WITHIN GROUP (
                ORDER BY abs(base.price_per_sqm-summary_inner.median)
            ) AS mad
        FROM sale_outlier_base base
        JOIN sale_outlier_municipality summary_inner
          USING (municipality_id,property_group,area_band,contract_year)
        GROUP BY 1,2,3,4
    ) deviation
    WHERE summary.municipality_id=deviation.municipality_id
      AND summary.property_group=deviation.property_group
      AND summary.area_band=deviation.area_band
      AND summary.contract_year=deviation.contract_year;

    DROP TABLE IF EXISTS pg_temp.sale_outlier_region;
    CREATE TEMP TABLE sale_outlier_region ON COMMIT DROP AS
    SELECT
        statistical_region_id,property_group,area_band,contract_year,
        count(*)::integer AS cohort_size,
        percentile_cont(0.25) WITHIN GROUP (ORDER BY price_per_sqm) AS q1,
        percentile_cont(0.5) WITHIN GROUP (ORDER BY price_per_sqm) AS median,
        percentile_cont(0.75) WITHIN GROUP (ORDER BY price_per_sqm) AS q3,
        NULL::numeric AS mad
    FROM sale_outlier_base
    WHERE statistical_region_id IS NOT NULL
    GROUP BY 1,2,3,4;
    UPDATE sale_outlier_region summary
    SET mad=deviation.mad
    FROM (
        SELECT
            base.statistical_region_id,base.property_group,base.area_band,base.contract_year,
            percentile_cont(0.5) WITHIN GROUP (
                ORDER BY abs(base.price_per_sqm-summary_inner.median)
            ) AS mad
        FROM sale_outlier_base base
        JOIN sale_outlier_region summary_inner
          USING (statistical_region_id,property_group,area_band,contract_year)
        GROUP BY 1,2,3,4
    ) deviation
    WHERE summary.statistical_region_id=deviation.statistical_region_id
      AND summary.property_group=deviation.property_group
      AND summary.area_band=deviation.area_band
      AND summary.contract_year=deviation.contract_year;

    TRUNCATE analytics.sale_outlier_evaluations;
    INSERT INTO analytics.sale_outlier_evaluations (
        transaction_id,status,method,cohort_level,cohort_size,cohort_median,
        lower_bound,upper_bound,modified_z_score,rule_version,reasons
    )
    SELECT
        fact.transaction_id,
        CASE
            WHEN NOT fact.is_quality_eligible THEN 'ineligible'
            WHEN base.transaction_id IS NULL THEN 'insufficient_cohort'
            WHEN chosen.cohort_size IS NULL THEN 'insufficient_cohort'
            WHEN analytics.is_statistical_outlier(
                fact.price_per_sqm,chosen.q1::numeric,chosen.q3::numeric,
                chosen.median::numeric,chosen.mad::numeric
            ) THEN 'outlier'
            ELSE 'inlier'
        END,
        CASE WHEN chosen.cohort_size IS NOT NULL THEN 'iqr_and_mad' END,
        chosen.cohort_level,chosen.cohort_size,chosen.median,
        chosen.q1-1.5*greatest(chosen.q3-chosen.q1,abs(chosen.median)*0.20),
        chosen.q3+1.5*greatest(chosen.q3-chosen.q1,abs(chosen.median)*0.20),
        CASE WHEN chosen.median>0
             THEN 0.6745*abs(fact.price_per_sqm-chosen.median)
                  /greatest(chosen.mad,abs(chosen.median)*0.10) END,
        'sale-outlier-v2',
        CASE
            WHEN NOT fact.is_quality_eligible THEN fact.quality_reasons
            WHEN base.transaction_id IS NULL THEN ARRAY['unsupported_or_missing_comparable']::text[]
            WHEN chosen.cohort_size IS NULL THEN ARRAY['insufficient_comparable_cohort']::text[]
            WHEN analytics.is_statistical_outlier(
                fact.price_per_sqm,chosen.q1::numeric,chosen.q3::numeric,
                chosen.median::numeric,chosen.mad::numeric
            ) THEN ARRAY['statistically_atypical_price_per_sqm']::text[]
            ELSE ARRAY[]::text[]
        END
    FROM analytics.sale_facts fact
    LEFT JOIN sale_outlier_base base USING (transaction_id)
    LEFT JOIN sale_outlier_building building
      ON building.cadastral_municipality_code=base.cadastral_municipality_code
     AND building.building_number=base.building_number
     AND building.property_group=base.property_group
     AND building.area_band=base.area_band
     AND building.contract_year=base.contract_year
    LEFT JOIN sale_outlier_municipality municipality
      ON municipality.municipality_id=base.municipality_id
     AND municipality.property_group=base.property_group
     AND municipality.area_band=base.area_band
     AND municipality.contract_year=base.contract_year
    LEFT JOIN sale_outlier_region region
      ON region.statistical_region_id=base.statistical_region_id
     AND region.property_group=base.property_group
     AND region.area_band=base.area_band
     AND region.contract_year=base.contract_year
    LEFT JOIN LATERAL (
        SELECT *
        FROM (
            SELECT 'building'::text AS cohort_level,building.cohort_size,
                   building.q1,building.median,building.q3,building.mad,1 AS priority
            WHERE building.cohort_size>=5
            UNION ALL
            SELECT 'municipality_year',municipality.cohort_size,
                   municipality.q1,municipality.median,municipality.q3,municipality.mad,2
            WHERE municipality.cohort_size>=15
            UNION ALL
            SELECT 'statistical_region_year',region.cohort_size,
                   region.q1,region.median,region.q3,region.mad,3
            WHERE region.cohort_size>=30
        ) cohorts ORDER BY priority LIMIT 1
    ) chosen ON true;

    UPDATE analytics.sale_facts fact
    SET representative_eligible=(
        fact.is_quality_eligible AND evaluation.status<>'outlier'
    )
    FROM analytics.sale_outlier_evaluations evaluation
    WHERE evaluation.transaction_id=fact.transaction_id;

    GET DIAGNOSTICS evaluated_rows=ROW_COUNT;
    ANALYZE analytics.sale_facts;
    ANALYZE analytics.sale_outlier_evaluations;
    UPDATE analytics.refresh_runs
    SET status='succeeded',finished_at=clock_timestamp(),
        details=jsonb_build_object('rows',evaluated_rows,'ruleVersion','sale-outlier-v2')
    WHERE id=refresh_run_id;
    RETURN evaluated_rows;
END
$$;

COMMENT ON TABLE analytics.sale_outlier_evaluations IS
    'Auditable robust price/m2 evaluation; observations are flagged but never deleted.';
COMMENT ON COLUMN analytics.sale_facts.representative_eligible IS
    'Passes deterministic quality rules and is not a detected statistical market outlier.';
