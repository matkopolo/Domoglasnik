CREATE TABLE IF NOT EXISTS analytics.repeat_sale_pairs (
    transaction_id bigint PRIMARY KEY REFERENCES analytics.sale_facts(transaction_id),
    previous_transaction_id bigint NOT NULL REFERENCES analytics.sale_facts(transaction_id),
    property_group text NOT NULL,
    current_contract_date date NOT NULL,
    previous_contract_date date NOT NULL,
    current_price_per_sqm numeric(18,4) NOT NULL,
    previous_price_per_sqm numeric(18,4) NOT NULL,
    years_between numeric(12,6) NOT NULL,
    annualized_log_return numeric(18,10) NOT NULL,
    cadastral_municipality_code integer NOT NULL,
    building_number text NOT NULL,
    building_part_number text NOT NULL
);
CREATE INDEX IF NOT EXISTS repeat_sale_pairs_group_date_idx
    ON analytics.repeat_sale_pairs(property_group,current_contract_date);

CREATE TABLE IF NOT EXISTS analytics.market_indices (
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    method text NOT NULL CHECK (method IN ('repeat_sales','hedonic')),
    property_group text NOT NULL,
    geography_id bigint REFERENCES analytics.geographic_units(id) ON DELETE CASCADE,
    period date NOT NULL,
    index_value numeric(18,6) NOT NULL,
    sample_size integer NOT NULL,
    base_period date NOT NULL,
    model_run_id uuid REFERENCES analytics.model_runs(id),
    methodology_version text NOT NULL,
    refreshed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT(
        transaction_kind,method,property_group,geography_id,period
    )
);

CREATE OR REPLACE FUNCTION analytics.refresh_repeat_sales_indices()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    inserted bigint;
BEGIN
    TRUNCATE analytics.repeat_sale_pairs;
    INSERT INTO analytics.repeat_sale_pairs
    WITH ordered AS (
        SELECT transaction_id,property_group,contract_date,price_per_sqm,
          cadastral_municipality_code,building_number,building_part_number,
          lag(transaction_id) OVER identity_order previous_transaction_id,
          lag(contract_date) OVER identity_order previous_date,
          lag(price_per_sqm) OVER identity_order previous_price_per_sqm
        FROM analytics.sale_facts
        WHERE representative_eligible AND property_group IN ('apartment','commercial','garage')
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

    DELETE FROM analytics.market_indices
    WHERE transaction_kind='sale' AND method='repeat_sales';
    INSERT INTO analytics.market_indices(
        transaction_kind,method,property_group,geography_id,period,index_value,
        sample_size,base_period,methodology_version
    )
    WITH annual AS (
      SELECT property_group,date_trunc('year',current_contract_date)::date period,
        percentile_cont(.5) WITHIN GROUP(ORDER BY annualized_log_return) growth,
        count(*)::integer sample_size
      FROM analytics.repeat_sale_pairs GROUP BY 1,2
    ), valid AS (
      SELECT * FROM annual WHERE sample_size>=30
    ), accumulated AS (
      SELECT *,sum(growth) OVER (
        PARTITION BY property_group ORDER BY period
      ) cumulative_growth,
      min(period) OVER(PARTITION BY property_group) base_period
      FROM valid
    ), indexed AS (
      SELECT *,exp(cumulative_growth-first_value(cumulative_growth) OVER (
        PARTITION BY property_group ORDER BY period
      ))*100 index_value
      FROM accumulated
    )
    SELECT 'sale','repeat_sales',property_group,NULL,period,
      index_value,sample_size,base_period,'repeat-sales-median-v1'
    FROM indexed;
    GET DIAGNOSTICS inserted=ROW_COUNT;
    ANALYZE analytics.repeat_sale_pairs;
    ANALYZE analytics.market_indices;
    RETURN inserted;
END
$$;

GRANT SELECT ON analytics.repeat_sale_pairs,analytics.market_indices
    TO real_estate_api;

COMMENT ON TABLE analytics.repeat_sale_pairs IS
    'Matched repeat transactions for the same cadastral building part.';
COMMENT ON TABLE analytics.market_indices IS
    'Versioned repeat-sales and constant-quality hedonic market indices.';
