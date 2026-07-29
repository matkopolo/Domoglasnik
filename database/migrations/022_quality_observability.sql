CREATE TABLE IF NOT EXISTS analytics.publication_policy (
    metric_family text PRIMARY KEY,
    minimum_sample_size integer NOT NULL CHECK (minimum_sample_size>0),
    policy_version text NOT NULL
);
INSERT INTO analytics.publication_policy(metric_family,minimum_sample_size,policy_version)
VALUES
    ('descriptive_statistics',30,'publication-v1'),
    ('market_index',30,'publication-v1')
ON CONFLICT(metric_family) DO UPDATE SET
    minimum_sample_size=excluded.minimum_sample_size,
    policy_version=excluded.policy_version;

CREATE TABLE IF NOT EXISTS analytics.missingness_profiles (
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    property_group text NOT NULL,
    contract_year integer,
    geography_level text NOT NULL DEFAULT 'national'
        CHECK(geography_level IN ('national','municipality')),
    geography_id bigint NOT NULL DEFAULT 0,
    field_name text NOT NULL,
    total_rows bigint NOT NULL,
    missing_rows bigint NOT NULL,
    missing_rate numeric(10,8) NOT NULL,
    data_vintage timestamptz NOT NULL,
    PRIMARY KEY(
        transaction_kind,property_group,contract_year,
        geography_level,geography_id,field_name
    )
);
ALTER TABLE analytics.missingness_profiles
    ADD COLUMN IF NOT EXISTS geography_level text NOT NULL DEFAULT 'national',
    ADD COLUMN IF NOT EXISTS geography_id bigint NOT NULL DEFAULT 0;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid='analytics.missingness_profiles'::regclass
          AND conname='missingness_profiles_geography_level_check'
    ) THEN
        ALTER TABLE analytics.missingness_profiles
          ADD CONSTRAINT missingness_profiles_geography_level_check
          CHECK(geography_level IN ('national','municipality'));
    END IF;
END
$$;
ALTER TABLE analytics.missingness_profiles
    DROP CONSTRAINT IF EXISTS missingness_profiles_pkey;
ALTER TABLE analytics.missingness_profiles
    ADD PRIMARY KEY(
        transaction_kind,property_group,contract_year,
        geography_level,geography_id,field_name
    );

CREATE OR REPLACE FUNCTION analytics.refresh_missingness_profiles()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    affected bigint;
BEGIN
    TRUNCATE analytics.missingness_profiles;
    INSERT INTO analytics.missingness_profiles(
      transaction_kind,property_group,contract_year,
      geography_level,geography_id,field_name,total_rows,missing_rows,
      missing_rate,data_vintage
    )
    WITH facts AS (
      SELECT 'sale'::text kind,property_group,
        extract(year FROM contract_date)::integer contract_year,
        contract_date,area,price amount,price_per_sqm amount_per_sqm,
        location,municipality_id
      FROM analytics.sale_facts
      UNION ALL
      SELECT 'rent',property_group,
        extract(year FROM contract_date)::integer,
        contract_date,area,monthly_rent_ex_vat,monthly_rent_per_sqm,
        location,municipality_id
      FROM analytics.rental_facts
    ), fields AS (
      SELECT kind,property_group,contract_year,
        geography.level geography_level,geography.id geography_id,
        field_name,is_missing
      FROM facts
      CROSS JOIN LATERAL (VALUES
        ('national'::text,0::bigint),
        ('municipality'::text,municipality_id)
      ) geography(level,id)
      CROSS JOIN LATERAL (VALUES
        ('contract_date',contract_date IS NULL),
        ('area',area IS NULL),
        ('amount',amount IS NULL),
        ('amount_per_sqm',amount_per_sqm IS NULL),
        ('location',location IS NULL),
        ('municipality',municipality_id IS NULL)
      ) value(field_name,is_missing)
      WHERE geography.level='national' OR geography.id IS NOT NULL
    )
    SELECT kind,property_group,contract_year,geography_level,geography_id,
      field_name,count(*),
      count(*) FILTER(WHERE is_missing),
      count(*) FILTER(WHERE is_missing)::numeric/count(*),now()
    FROM fields
    GROUP BY kind,property_group,contract_year,
      geography_level,geography_id,field_name;
    GET DIAGNOSTICS affected=ROW_COUNT;
    ANALYZE analytics.missingness_profiles;
    RETURN affected;
END
$$;

CREATE TABLE IF NOT EXISTS analytics.near_duplicate_candidates (
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    transaction_id bigint NOT NULL,
    candidate_transaction_id bigint NOT NULL,
    identity_basis text NOT NULL,
    days_apart integer NOT NULL,
    relative_area_difference numeric(12,8) NOT NULL,
    relative_amount_difference numeric(12,8) NOT NULL,
    candidate_score numeric(8,6) NOT NULL CHECK(candidate_score BETWEEN 0 AND 1),
    rule_version text NOT NULL,
    evaluated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(transaction_kind,transaction_id,candidate_transaction_id),
    CHECK(transaction_id<candidate_transaction_id)
);
CREATE INDEX IF NOT EXISTS near_duplicate_candidates_transaction_idx
    ON analytics.near_duplicate_candidates(transaction_kind,transaction_id);

CREATE OR REPLACE FUNCTION analytics.refresh_near_duplicate_candidates()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    affected bigint;
BEGIN
    TRUNCATE analytics.near_duplicate_candidates;
    INSERT INTO analytics.near_duplicate_candidates
    WITH facts AS (
      SELECT 'sale'::text kind,transaction_id,contract_date,area,
        price amount,cadastral_municipality_code,building_number,
        building_part_number,property_group
      FROM analytics.sale_facts
      UNION ALL
      SELECT 'rent',transaction_id,contract_date,area,
        monthly_rent_ex_vat,cadastral_municipality_code,building_number,
        building_part_number,property_group
      FROM analytics.rental_facts
    )
    SELECT left_fact.kind,left_fact.transaction_id,right_fact.transaction_id,
      'same_building_part',
      abs(left_fact.contract_date-right_fact.contract_date),
      abs(left_fact.area-right_fact.area)/greatest(left_fact.area,right_fact.area),
      abs(left_fact.amount-right_fact.amount)/greatest(left_fact.amount,right_fact.amount),
      greatest(0,1-
        abs(left_fact.contract_date-right_fact.contract_date)/30.0-
        abs(left_fact.area-right_fact.area)/greatest(left_fact.area,right_fact.area)-
        abs(left_fact.amount-right_fact.amount)/greatest(left_fact.amount,right_fact.amount)
      ),
      'near-duplicate-v1',now()
    FROM facts left_fact
    JOIN facts right_fact
      ON right_fact.kind=left_fact.kind
     AND right_fact.transaction_id>left_fact.transaction_id
     AND right_fact.property_group=left_fact.property_group
     AND right_fact.cadastral_municipality_code=left_fact.cadastral_municipality_code
     AND right_fact.building_number=left_fact.building_number
     AND right_fact.building_part_number=left_fact.building_part_number
     AND right_fact.contract_date BETWEEN
         left_fact.contract_date-30 AND left_fact.contract_date+30
    WHERE left_fact.contract_date IS NOT NULL
      AND left_fact.area>0 AND right_fact.area>0
      AND left_fact.amount>0 AND right_fact.amount>0
      AND abs(left_fact.area-right_fact.area)/
          greatest(left_fact.area,right_fact.area)<=.02
      AND abs(left_fact.amount-right_fact.amount)/
          greatest(left_fact.amount,right_fact.amount)<=.02;
    GET DIAGNOSTICS affected=ROW_COUNT;

    UPDATE analytics.anomaly_signals signal
    SET duplicate_score=greatest(signal.duplicate_score,candidate.candidate_score),
        flags=CASE WHEN 'possible_near_duplicate'=ANY(signal.flags)
          THEN signal.flags ELSE array_append(signal.flags,'possible_near_duplicate') END,
        review_score=least(1.0,
          .35*signal.hard_rule_score+.20*signal.robust_cohort_score+
          .20*coalesce(signal.model_residual_score,0)+
          .10*coalesce(signal.isolation_forest_score,0)+
          .10*greatest(signal.duplicate_score,candidate.candidate_score)+
          .05*signal.spatial_score)
    FROM (
      SELECT transaction_kind,transaction_id,max(candidate_score) candidate_score
      FROM (
        SELECT transaction_kind,transaction_id,candidate_score
        FROM analytics.near_duplicate_candidates
        UNION ALL
        SELECT transaction_kind,candidate_transaction_id,candidate_score
        FROM analytics.near_duplicate_candidates
      ) candidates GROUP BY 1,2
    ) candidate
    WHERE candidate.transaction_kind=signal.transaction_kind
      AND candidate.transaction_id=signal.transaction_id;
    ANALYZE analytics.near_duplicate_candidates;
    RETURN affected;
END
$$;

SELECT analytics.refresh_missingness_profiles();

GRANT SELECT ON analytics.publication_policy,
    analytics.missingness_profiles,analytics.near_duplicate_candidates
TO real_estate_api;

COMMENT ON TABLE analytics.missingness_profiles IS
    'Versioned aggregate input coverage; target values are never imputed.';
COMMENT ON TABLE analytics.near_duplicate_candidates IS
    'Explainable cross-transaction candidates; multi-component rows inside one transaction are never candidates.';
