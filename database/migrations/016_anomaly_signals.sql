CREATE TABLE IF NOT EXISTS analytics.anomaly_signals (
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    transaction_id bigint NOT NULL,
    flags text[] NOT NULL DEFAULT ARRAY[]::text[],
    hard_rule_score numeric(8,6) NOT NULL CHECK (hard_rule_score BETWEEN 0 AND 1),
    robust_cohort_score numeric(8,6) NOT NULL CHECK (robust_cohort_score BETWEEN 0 AND 1),
    model_residual_score numeric(8,6) CHECK (model_residual_score BETWEEN 0 AND 1),
    isolation_forest_score numeric(8,6) CHECK (isolation_forest_score BETWEEN 0 AND 1),
    duplicate_score numeric(8,6) NOT NULL CHECK (duplicate_score BETWEEN 0 AND 1),
    spatial_score numeric(8,6) NOT NULL CHECK (spatial_score BETWEEN 0 AND 1),
    review_score numeric(8,6) NOT NULL CHECK (review_score BETWEEN 0 AND 1),
    signal_version text NOT NULL,
    model_version text,
    review_status text NOT NULL DEFAULT 'unreviewed' CHECK (review_status IN (
        'unreviewed','confirmed_valid','confirmed_error','needs_follow_up','ignored'
    )),
    review_notes text,
    reviewed_by text,
    reviewed_at timestamptz,
    evaluated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(transaction_kind,transaction_id)
);

CREATE INDEX IF NOT EXISTS anomaly_signals_queue_idx
    ON analytics.anomaly_signals(review_status,review_score DESC);
CREATE INDEX IF NOT EXISTS anomaly_signals_flags_gin_idx
    ON analytics.anomaly_signals USING gin(flags);

CREATE OR REPLACE FUNCTION analytics.refresh_anomaly_signals()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    affected bigint;
    refresh_run_id bigint;
BEGIN
    INSERT INTO analytics.refresh_runs(process,status)
    VALUES ('anomaly_signals','running') RETURNING id INTO refresh_run_id;

    WITH sale_duplicates AS (
        SELECT transaction_id,count(*) OVER (
            PARTITION BY contract_date,property_group,area,price,
                         cadastral_municipality_code,building_number,
                         building_part_number
        ) duplicate_count
        FROM analytics.sale_facts
        WHERE contract_date IS NOT NULL AND area IS NOT NULL AND price IS NOT NULL
    ), rental_duplicates AS (
        SELECT transaction_id,count(*) OVER (
            PARTITION BY contract_date,property_group,area,monthly_rent_ex_vat,
                         cadastral_municipality_code,building_number,
                         building_part_number
        ) duplicate_count
        FROM analytics.rental_facts
        WHERE contract_date IS NOT NULL AND area IS NOT NULL
          AND monthly_rent_ex_vat IS NOT NULL
    ), raw_signals AS (
        SELECT 'sale'::text transaction_kind,f.transaction_id,
          array_cat(f.quality_reasons,array_remove(ARRAY[
            CASE WHEN e.status='outlier' THEN 'extreme_price_per_m2' END,
            CASE WHEN coalesce(d.duplicate_count,0)>1 THEN 'possible_duplicate' END,
            CASE WHEN f.location IS NULL THEN 'missing_coordinates' END,
            CASE WHEN f.location IS NOT NULL AND NOT EXISTS (
              SELECT 1 FROM analytics.geographic_units country
              WHERE country.level='country'
                AND ST_Covers(country.geometry_d96tm,f.location)
            ) THEN 'coordinate_outside_slovenia' END,
            CASE WHEN t.municipality_code IS NOT NULL AND municipality.code IS NOT NULL
                      AND t.municipality_code<>municipality.code
                 THEN 'coordinate_outside_municipality' END
          ],NULL)) flags,
          least(1.0,cardinality(f.quality_reasons)/2.0)::numeric hard_score,
          CASE WHEN e.status='outlier'
               THEN least(1.0,coalesce(e.modified_z_score,5)/10.0)
               ELSE 0 END::numeric robust_score,
          CASE WHEN coalesce(d.duplicate_count,0)>1 THEN 1 ELSE 0 END::numeric duplicate_score,
          CASE WHEN f.location IS NULL THEN 0.5
               WHEN t.municipality_code IS NOT NULL AND municipality.code IS NOT NULL
                    AND t.municipality_code<>municipality.code THEN 1
               WHEN NOT EXISTS (
                 SELECT 1 FROM analytics.geographic_units country
                 WHERE country.level='country'
                   AND ST_Covers(country.geometry_d96tm,f.location)
               ) THEN 1 ELSE 0 END::numeric spatial_score
        FROM analytics.sale_facts f
        JOIN transactions t USING(transaction_id)
        LEFT JOIN analytics.sale_outlier_evaluations e USING(transaction_id)
        LEFT JOIN sale_duplicates d USING(transaction_id)
        LEFT JOIN analytics.geographic_units municipality
          ON municipality.id=f.municipality_id
        UNION ALL
        SELECT 'rent',f.transaction_id,
          array_cat(f.quality_reasons,array_remove(ARRAY[
            CASE WHEN e.status='outlier' THEN 'extreme_price_per_m2' END,
            CASE WHEN coalesce(d.duplicate_count,0)>1 THEN 'possible_duplicate' END,
            CASE WHEN f.location IS NULL THEN 'missing_coordinates' END,
            CASE WHEN f.location IS NOT NULL AND NOT EXISTS (
              SELECT 1 FROM analytics.geographic_units country
              WHERE country.level='country'
                AND ST_Covers(country.geometry_d96tm,f.location)
            ) THEN 'coordinate_outside_slovenia' END
          ],NULL)),
          least(1.0,cardinality(f.quality_reasons)/2.0)::numeric,
          CASE WHEN e.status='outlier'
               THEN least(1.0,coalesce(e.modified_z_score,5)/10.0)
               ELSE 0 END::numeric,
          CASE WHEN coalesce(d.duplicate_count,0)>1 THEN 1 ELSE 0 END::numeric,
          CASE WHEN f.location IS NULL THEN 0.5
               WHEN NOT EXISTS (
                 SELECT 1 FROM analytics.geographic_units country
                 WHERE country.level='country'
                   AND ST_Covers(country.geometry_d96tm,f.location)
               ) THEN 1 ELSE 0 END::numeric
        FROM analytics.rental_facts f
        LEFT JOIN analytics.rental_outlier_evaluations e USING(transaction_id)
        LEFT JOIN rental_duplicates d USING(transaction_id)
    ), scored AS (
        SELECT *,least(1.0,
          0.35*hard_score+0.20*robust_score+
          0.10*duplicate_score+0.05*spatial_score
        )::numeric AS review_score
        FROM raw_signals
    )
    INSERT INTO analytics.anomaly_signals(
        transaction_kind,transaction_id,flags,hard_rule_score,
        robust_cohort_score,duplicate_score,spatial_score,review_score,
        signal_version,evaluated_at
    )
    SELECT transaction_kind,transaction_id,flags,hard_score,robust_score,
           duplicate_score,spatial_score,review_score,'anomaly-signals-v1',now()
    FROM scored
    ON CONFLICT(transaction_kind,transaction_id) DO UPDATE SET
        flags=excluded.flags,
        hard_rule_score=excluded.hard_rule_score,
        robust_cohort_score=excluded.robust_cohort_score,
        duplicate_score=excluded.duplicate_score,
        spatial_score=excluded.spatial_score,
        review_score=excluded.review_score,
        signal_version=excluded.signal_version,
        evaluated_at=excluded.evaluated_at;

    GET DIAGNOSTICS affected=ROW_COUNT;

    DELETE FROM analytics.anomaly_signals signal
    WHERE (signal.transaction_kind='sale' AND NOT EXISTS (
        SELECT 1 FROM analytics.sale_facts f
        WHERE f.transaction_id=signal.transaction_id
    )) OR (signal.transaction_kind='rent' AND NOT EXISTS (
        SELECT 1 FROM analytics.rental_facts f
        WHERE f.transaction_id=signal.transaction_id
    ));

    ANALYZE analytics.anomaly_signals;
    UPDATE analytics.refresh_runs
    SET status='succeeded',finished_at=clock_timestamp(),
        details=jsonb_build_object('rows',affected,'signalVersion','anomaly-signals-v1')
    WHERE id=refresh_run_id;
    RETURN affected;
END
$$;

COMMENT ON TABLE analytics.anomaly_signals IS
    'Non-destructive, versioned anomaly signals and persistent human review state.';
