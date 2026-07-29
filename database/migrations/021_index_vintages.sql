CREATE TABLE IF NOT EXISTS analytics.index_calculation_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    method text NOT NULL,
    methodology_version text NOT NULL,
    data_from date,
    data_to date,
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE analytics.market_indices
    DROP CONSTRAINT IF EXISTS market_indices_method_check;

UPDATE analytics.market_indices
SET method='matched_pair_median',
    methodology_version='matched-pair-median-v2'
WHERE method='repeat_sales';

ALTER TABLE analytics.market_indices
    ADD CONSTRAINT market_indices_method_check
    CHECK (method IN ('matched_pair_median','hedonic'));

ALTER TABLE analytics.market_indices
    ADD COLUMN IF NOT EXISTS calculation_run_id uuid
        REFERENCES analytics.index_calculation_runs(id),
    ADD COLUMN IF NOT EXISTS observed_sample_size integer,
    ADD COLUMN IF NOT EXISTS reference_sample_size integer,
    ADD COLUMN IF NOT EXISTS lower_index_value numeric(18,6),
    ADD COLUMN IF NOT EXISTS upper_index_value numeric(18,6),
    ADD COLUMN IF NOT EXISTS nominal_coverage numeric(6,5),
    ADD COLUMN IF NOT EXISTS uncertainty_method text,
    ADD COLUMN IF NOT EXISTS is_publishable boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS suppression_reason text;

DO $$
DECLARE
    legacy_run uuid;
    item record;
BEGIN
    FOR item IN
        SELECT DISTINCT transaction_kind,method,methodology_version
        FROM analytics.market_indices
        WHERE calculation_run_id IS NULL
    LOOP
        INSERT INTO analytics.index_calculation_runs(
            transaction_kind,method,methodology_version,parameters
        ) VALUES(
            item.transaction_kind,item.method,item.methodology_version,
            '{"legacyBackfill":true}'::jsonb
        ) RETURNING id INTO legacy_run;
        UPDATE analytics.market_indices
        SET calculation_run_id=legacy_run,
            observed_sample_size=sample_size,
            is_publishable=sample_size>=30,
            suppression_reason=CASE WHEN sample_size<30
              THEN 'below_minimum_sample_30' END
        WHERE transaction_kind=item.transaction_kind
          AND method=item.method
          AND methodology_version=item.methodology_version
          AND calculation_run_id IS NULL;
    END LOOP;
END
$$;

ALTER TABLE analytics.market_indices
    ALTER COLUMN calculation_run_id SET NOT NULL,
    ALTER COLUMN observed_sample_size SET NOT NULL;

ALTER TABLE analytics.market_indices
    DROP CONSTRAINT IF EXISTS market_indices_transaction_kind_method_property_group_geogr_key;
CREATE UNIQUE INDEX IF NOT EXISTS market_indices_vintage_unique
    ON analytics.market_indices(
        calculation_run_id,property_group,geography_id,period
    ) NULLS NOT DISTINCT;

CREATE OR REPLACE VIEW analytics.current_market_indices AS
SELECT DISTINCT ON (
    index_data.transaction_kind,index_data.method,index_data.property_group,
    index_data.geography_id,index_data.period
) index_data.*
FROM analytics.market_indices index_data
JOIN analytics.index_calculation_runs run
  ON run.id=index_data.calculation_run_id
ORDER BY index_data.transaction_kind,index_data.method,index_data.property_group,
  index_data.geography_id,index_data.period,run.created_at DESC,run.id DESC;

CREATE OR REPLACE FUNCTION analytics.refresh_repeat_sales_indices()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    inserted bigint;
    run_id uuid;
BEGIN
    TRUNCATE analytics.repeat_sale_pairs;
    INSERT INTO analytics.repeat_sale_pairs
    WITH ordered AS (
        SELECT transaction_id,property_group,contract_date,price_per_sqm,
          cadastral_municipality_code,building_number,building_part_number,
          lag(transaction_id) OVER identity_order previous_transaction_id,
          lag(contract_date) OVER identity_order previous_date,
          lag(price_per_sqm) OVER identity_order previous_price_per_sqm
        FROM analytics.sale_facts_strict_land
        WHERE representative_eligible
          AND property_group IN ('apartment','commercial','garage')
          AND cadastral_municipality_code IS NOT NULL
          AND building_number IS NOT NULL AND building_part_number IS NOT NULL
          AND contract_date IS NOT NULL AND price_per_sqm>0
        WINDOW identity_order AS (
          PARTITION BY cadastral_municipality_code,building_number,building_part_number
          ORDER BY contract_date,transaction_id
        )
    )
    SELECT transaction_id,previous_transaction_id,property_group,contract_date,
      previous_date,price_per_sqm,previous_price_per_sqm,
      (contract_date-previous_date)/365.2425,
      ln(price_per_sqm/previous_price_per_sqm)/
        ((contract_date-previous_date)/365.2425),
      cadastral_municipality_code,building_number,building_part_number
    FROM ordered
    WHERE previous_transaction_id IS NOT NULL
      AND contract_date>previous_date
      AND contract_date-previous_date>=180
      AND price_per_sqm/previous_price_per_sqm BETWEEN 0.1 AND 10;

    INSERT INTO analytics.index_calculation_runs(
        transaction_kind,method,methodology_version,data_from,data_to,parameters
    )
    SELECT 'sale','matched_pair_median','matched-pair-median-v2',
      min(previous_contract_date),max(current_contract_date),
      '{"minimumPairs":30,"uncertainty":"pair_return_quantiles","nominalCoverage":0.95}'::jsonb
    FROM analytics.repeat_sale_pairs
    RETURNING id INTO run_id;

    INSERT INTO analytics.market_indices(
        transaction_kind,method,property_group,geography_id,period,index_value,
        sample_size,base_period,methodology_version,calculation_run_id,
        observed_sample_size,lower_index_value,upper_index_value,
        nominal_coverage,uncertainty_method,is_publishable,suppression_reason
    )
    WITH annual AS (
      SELECT property_group,date_trunc('year',current_contract_date)::date period,
        percentile_cont(.5) WITHIN GROUP(ORDER BY annualized_log_return) growth,
        percentile_cont(.025) WITHIN GROUP(ORDER BY annualized_log_return) low_growth,
        percentile_cont(.975) WITHIN GROUP(ORDER BY annualized_log_return) high_growth,
        count(*)::integer observed
      FROM analytics.repeat_sale_pairs GROUP BY 1,2
    ), accumulated AS (
      SELECT *,
        sum(growth) OVER (PARTITION BY property_group ORDER BY period) cumulative,
        sum(low_growth) OVER (PARTITION BY property_group ORDER BY period) low_cumulative,
        sum(high_growth) OVER (PARTITION BY property_group ORDER BY period) high_cumulative,
        min(period) OVER(PARTITION BY property_group) base_period
      FROM annual
    )
    SELECT 'sale','matched_pair_median',property_group,NULL,period,
      exp(cumulative-first_value(cumulative) OVER group_order)*100,
      observed,base_period,'matched-pair-median-v2',run_id,observed,
      exp(low_cumulative-first_value(low_cumulative) OVER group_order)*100,
      exp(high_cumulative-first_value(high_cumulative) OVER group_order)*100,
      .95,'empirical_pair_return_quantiles',observed>=30,
      CASE WHEN observed<30 THEN 'below_minimum_pair_count_30' END
    FROM accumulated
    WINDOW group_order AS (PARTITION BY property_group ORDER BY period);
    GET DIAGNOSTICS inserted=ROW_COUNT;
    ANALYZE analytics.repeat_sale_pairs;
    ANALYZE analytics.market_indices;
    RETURN inserted;
END
$$;

GRANT SELECT ON analytics.index_calculation_runs,
    analytics.current_market_indices TO real_estate_api;

COMMENT ON TABLE analytics.index_calculation_runs IS
    'Append-only calculation vintages and parameters for reproducible market indices.';
COMMENT ON VIEW analytics.current_market_indices IS
    'Latest available calculation vintage for each market-index series and period.';
