-- Browsing categories only. Do not alter strict/broad model populations or prices.
BEGIN;
CREATE OR REPLACE FUNCTION analytics.land_market_category(codes integer[], unknown_count integer)
RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
    WITH categories AS (
        SELECT CASE
            WHEN code IN (1,2) THEN 'strict'
            WHEN code=3 THEN 'unverified'
            WHEN code IN (7,8) THEN 'agricultural'
            WHEN code=9 THEN 'forest'
            ELSE 'other'
        END AS category
        FROM unnest(coalesce(codes,ARRAY[]::integer[])) code
        UNION ALL
        SELECT 'other' WHERE coalesce(unknown_count,0)>0
    )
    SELECT CASE WHEN count(DISTINCT category)>1 THEN 'mixed'
                ELSE coalesce(min(category),'other') END
    FROM categories
$$;

COMMENT ON FUNCTION analytics.land_market_category(integer[],integer) IS
    'ETN codebook 3: 1/2 verified buildable, 3 unverified, 7/8 agricultural, 9 forest, others/unknown other. Contracts spanning categories are mixed. Browsing only.';

CREATE OR REPLACE VIEW analytics.land_market_categories AS
SELECT transaction_id,
       analytics.land_market_category(land_type_codes,unknown_parcel_count) AS category
FROM analytics.land_transaction_facts;

CREATE OR REPLACE VIEW analytics.sale_facts_agricultural_land AS
SELECT fact.* FROM analytics.sale_facts fact
WHERE fact.property_group<>'land' OR EXISTS (
    SELECT 1 FROM analytics.land_market_categories land
    WHERE land.transaction_id=fact.transaction_id AND land.category='agricultural'
);
CREATE OR REPLACE VIEW analytics.sale_facts_forest_land AS
SELECT fact.* FROM analytics.sale_facts fact
WHERE fact.property_group<>'land' OR EXISTS (
    SELECT 1 FROM analytics.land_market_categories land
    WHERE land.transaction_id=fact.transaction_id AND land.category='forest'
);
CREATE OR REPLACE VIEW analytics.sale_facts_unverified_land AS
SELECT fact.* FROM analytics.sale_facts fact
WHERE fact.property_group<>'land' OR EXISTS (
    SELECT 1 FROM analytics.land_market_categories land
    WHERE land.transaction_id=fact.transaction_id AND land.category='unverified'
);
CREATE OR REPLACE VIEW analytics.sale_facts_mixed_land AS
SELECT fact.* FROM analytics.sale_facts fact
WHERE fact.property_group<>'land' OR EXISTS (
    SELECT 1 FROM analytics.land_market_categories land
    WHERE land.transaction_id=fact.transaction_id AND land.category='mixed'
);
CREATE OR REPLACE VIEW analytics.sale_facts_other_land AS
SELECT fact.* FROM analytics.sale_facts fact
WHERE fact.property_group<>'land'
   OR (fact.property_group='land' AND NOT EXISTS (
       SELECT 1 FROM analytics.land_market_categories land
       WHERE land.transaction_id=fact.transaction_id AND land.category<>'other'
   ));

GRANT SELECT ON analytics.land_market_categories,
    analytics.sale_facts_agricultural_land,analytics.sale_facts_forest_land,
    analytics.sale_facts_unverified_land,analytics.sale_facts_mixed_land,
    analytics.sale_facts_other_land TO real_estate_api;
GRANT EXECUTE ON FUNCTION analytics.land_market_category(integer[],integer) TO real_estate_api;
COMMIT;
