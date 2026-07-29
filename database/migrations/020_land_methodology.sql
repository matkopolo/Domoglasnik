CREATE TABLE IF NOT EXISTS analytics.land_transaction_facts (
    transaction_id bigint PRIMARY KEY
        REFERENCES analytics.sale_facts(transaction_id) ON DELETE CASCADE,
    parcel_count integer NOT NULL,
    land_type_codes integer[] NOT NULL,
    readiness_class text NOT NULL CHECK (readiness_class IN (
        'building_permit','serviced_buildable','strict_mixed',
        'broad_unverified','mixed_or_non_buildable','unknown'
    )),
    is_strict_buildable boolean NOT NULL,
    is_broad_buildable boolean NOT NULL,
    parcel_area numeric(16,2),
    analytical_price numeric(16,2),
    price_per_sqm numeric(18,4),
    strict_parcel_count integer NOT NULL,
    broad_parcel_count integer NOT NULL,
    unknown_parcel_count integer NOT NULL,
    refreshed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS land_transaction_facts_scope_idx
    ON analytics.land_transaction_facts(is_strict_buildable,is_broad_buildable);

CREATE OR REPLACE FUNCTION analytics.refresh_land_transaction_facts()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    imported_rows bigint;
BEGIN
    TRUNCATE analytics.land_transaction_facts;
    INSERT INTO analytics.land_transaction_facts(
        transaction_id,parcel_count,land_type_codes,readiness_class,
        is_strict_buildable,is_broad_buildable,parcel_area,analytical_price,
        price_per_sqm,strict_parcel_count,broad_parcel_count,
        unknown_parcel_count
    )
    WITH parcels AS (
        SELECT lp.transaction_id,
          count(*)::integer parcel_count,
          array_agg(DISTINCT lp.land_type ORDER BY lp.land_type)
            FILTER (WHERE lp.land_type IS NOT NULL) land_type_codes,
          count(*) FILTER (WHERE lp.land_type IN (1,2))::integer strict_count,
          count(*) FILTER (WHERE lp.land_type IN (1,2,3))::integer broad_count,
          count(*) FILTER (WHERE lp.land_type IS NULL)::integer unknown_count
        FROM land_parcels lp
        GROUP BY lp.transaction_id
    )
    SELECT sf.transaction_id,p.parcel_count,
      coalesce(p.land_type_codes,ARRAY[]::integer[]),
      CASE
        WHEN p.unknown_count>0 OR cardinality(p.land_type_codes)=0 THEN 'unknown'
        WHEN p.strict_count=p.parcel_count
          AND p.land_type_codes=ARRAY[1] THEN 'building_permit'
        WHEN p.strict_count=p.parcel_count
          AND p.land_type_codes=ARRAY[2] THEN 'serviced_buildable'
        WHEN p.strict_count=p.parcel_count THEN 'strict_mixed'
        WHEN p.broad_count=p.parcel_count THEN 'broad_unverified'
        ELSE 'mixed_or_non_buildable'
      END,
      p.unknown_count=0 AND p.strict_count=p.parcel_count,
      p.unknown_count=0 AND p.broad_count=p.parcel_count,
      sf.area,sf.price,sf.price_per_sqm,
      p.strict_count,p.broad_count,p.unknown_count
    FROM analytics.sale_facts sf
    JOIN parcels p USING(transaction_id)
    WHERE sf.property_group='land';
    GET DIAGNOSTICS imported_rows=ROW_COUNT;
    ANALYZE analytics.land_transaction_facts;
    RETURN imported_rows;
END
$$;

CREATE OR REPLACE VIEW analytics.sale_facts_strict_land AS
SELECT fact.*
FROM analytics.sale_facts fact
WHERE fact.property_group<>'land'
   OR EXISTS (
       SELECT 1 FROM analytics.land_transaction_facts land
       WHERE land.transaction_id=fact.transaction_id
         AND land.is_strict_buildable
   );

CREATE OR REPLACE VIEW analytics.sale_facts_broad_land AS
SELECT fact.*
FROM analytics.sale_facts fact
WHERE fact.property_group<>'land'
   OR EXISTS (
       SELECT 1 FROM analytics.land_transaction_facts land
       WHERE land.transaction_id=fact.transaction_id
         AND land.is_broad_buildable
   );

CREATE OR REPLACE VIEW analytics.sale_facts_all_land AS
SELECT * FROM analytics.sale_facts;

SELECT analytics.refresh_land_transaction_facts();

GRANT SELECT ON analytics.land_transaction_facts,
    analytics.sale_facts_strict_land,
    analytics.sale_facts_broad_land,
    analytics.sale_facts_all_land
TO real_estate_api;

COMMENT ON TABLE analytics.land_transaction_facts IS
    'One row per land-only sale contract; all land is retained and classified by ETN readiness.';
COMMENT ON COLUMN analytics.land_transaction_facts.is_strict_buildable IS
    'True only when every parcel is verified-buildable ETN land type 1 or 2.';
COMMENT ON VIEW analytics.sale_facts_strict_land IS
    'Default analytical sale population: non-land facts plus land contracts containing only ETN codes 1 and 2.';
COMMENT ON VIEW analytics.sale_facts_broad_land IS
    'Opt-in population allowing land contracts containing only ETN codes 1, 2, and 3.';
