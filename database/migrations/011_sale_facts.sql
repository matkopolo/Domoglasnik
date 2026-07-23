CREATE TABLE IF NOT EXISTS analytics.sale_facts (
    transaction_id bigint PRIMARY KEY REFERENCES transactions(transaction_id) ON DELETE CASCADE,
    contract_date date,
    dataset_year integer NOT NULL,
    property_group text NOT NULL CHECK (property_group IN (
        'house', 'apartment', 'land', 'commercial', 'garage', 'mixed', 'unknown'
    )),
    building_part_count integer NOT NULL,
    land_parcel_count integer NOT NULL,
    primary_property_count integer NOT NULL,
    area numeric(16,2),
    price numeric(16,2),
    price_per_sqm numeric(18,4),
    price_derivation text NOT NULL CHECK (price_derivation IN (
        'component_prices', 'single_primary_transaction_price', 'unavailable'
    )),
    is_quality_eligible boolean NOT NULL,
    quality_reasons text[] NOT NULL DEFAULT ARRAY[]::text[],
    location geometry(Point,3794),
    statistical_region_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    municipality_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    city_district_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    local_community_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    settlement_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    refreshed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sale_facts_contract_date_idx ON analytics.sale_facts (contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_dataset_year_idx ON analytics.sale_facts (dataset_year);
CREATE INDEX IF NOT EXISTS sale_facts_property_group_date_idx ON analytics.sale_facts (property_group,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_region_date_idx ON analytics.sale_facts (statistical_region_id,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_municipality_date_idx ON analytics.sale_facts (municipality_id,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_city_district_date_idx ON analytics.sale_facts (city_district_id,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_local_community_date_idx ON analytics.sale_facts (local_community_id,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_settlement_date_idx ON analytics.sale_facts (settlement_id,contract_date);
CREATE INDEX IF NOT EXISTS sale_facts_location_gist_idx ON analytics.sale_facts USING gist (location);

CREATE OR REPLACE FUNCTION analytics.refresh_sale_facts()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    imported_rows bigint;
    refresh_run_id bigint;
BEGIN
    INSERT INTO analytics.refresh_runs(process,status)
    VALUES ('sale_facts','running') RETURNING id INTO refresh_run_id;
    TRUNCATE analytics.sale_facts;

    INSERT INTO analytics.sale_facts (
        transaction_id,contract_date,dataset_year,property_group,
        building_part_count,land_parcel_count,primary_property_count,
        area,price,price_per_sqm,price_derivation,is_quality_eligible,
        quality_reasons,location,statistical_region_id,municipality_id,
        city_district_id,local_community_id,settlement_id
    )
    WITH building AS (
        SELECT
            bp.transaction_id,
            count(*)::integer AS component_count,
            count(*) FILTER (WHERE bp.contractual_price>0)::integer AS priced_count,
            sum(bp.contractual_price) FILTER (WHERE bp.contractual_price>0) AS component_price,
            array_remove(array_agg(DISTINCT bp.building_part_type),NULL) AS source_types,
            count(*) FILTER (WHERE type_group.is_primary)::integer AS primary_count,
            sum(coalesce(bp.sold_area,bp.sold_building_part_area,bp.sold_usable_area))
                FILTER (WHERE type_group.is_primary) AS primary_area,
            sum(coalesce(bp.sold_area,bp.sold_building_part_area,bp.sold_usable_area))
                FILTER (WHERE type_group.property_group='garage') AS garage_area,
            ST_Centroid(ST_Collect(bp.location)
                FILTER (WHERE type_group.is_primary AND bp.location IS NOT NULL)) AS primary_location,
            ST_Centroid(ST_Collect(bp.location)
                FILTER (WHERE bp.location IS NOT NULL)) AS any_location
        FROM building_parts bp
        LEFT JOIN analytics.building_part_type_groups type_group
          ON type_group.source_code=bp.building_part_type
        GROUP BY bp.transaction_id
    ),
    land AS (
        SELECT
            lp.transaction_id,
            count(*)::integer AS component_count,
            count(*) FILTER (WHERE lp.contractual_price>0)::integer AS priced_count,
            sum(lp.contractual_price) FILTER (WHERE lp.contractual_price>0) AS component_price,
            array_remove(array_agg(DISTINCT lp.land_type),NULL) AS source_types,
            sum(lp.parcel_area) FILTER (WHERE lp.parcel_area>0) AS land_area,
            ST_Centroid(ST_Collect(lp.location)
                FILTER (WHERE lp.location IS NOT NULL)) AS location
        FROM land_parcels lp
        GROUP BY lp.transaction_id
    ),
    packages AS (
        SELECT
            transaction.transaction_id,transaction.contract_date,
            transaction.dataset_year,transaction.contract_price,
            analytics.classify_sale_package(building.source_types,land.source_types) AS property_group,
            coalesce(building.component_count,0) AS building_part_count,
            coalesce(land.component_count,0) AS land_parcel_count,
            CASE
                WHEN coalesce(building.primary_count,0)>0 THEN building.primary_count
                WHEN coalesce(building.component_count,0)>0 THEN building.component_count
                ELSE coalesce(land.component_count,0)
            END AS primary_property_count,
            CASE analytics.classify_sale_package(building.source_types,land.source_types)
                WHEN 'house' THEN building.primary_area
                WHEN 'apartment' THEN building.primary_area
                WHEN 'commercial' THEN building.primary_area
                WHEN 'garage' THEN building.garage_area
                WHEN 'land' THEN land.land_area
            END AS area,
            coalesce(building.component_count,0)+coalesce(land.component_count,0) AS component_count,
            coalesce(building.priced_count,0)+coalesce(land.priced_count,0) AS priced_count,
            coalesce(building.component_price,0)+coalesce(land.component_price,0) AS component_price,
            coalesce(building.primary_location,building.any_location,land.location) AS location
        FROM transactions transaction
        LEFT JOIN building ON building.transaction_id=transaction.transaction_id
        LEFT JOIN land ON land.transaction_id=transaction.transaction_id
    ),
    priced AS (
        SELECT packages.*,
            CASE
                WHEN component_count>0 AND priced_count=component_count AND component_price>0
                    THEN component_price
                WHEN primary_property_count=1 AND property_group NOT IN ('mixed','unknown')
                     AND contract_price>0 THEN contract_price
            END AS analytical_price,
            CASE
                WHEN component_count>0 AND priced_count=component_count AND component_price>0
                    THEN 'component_prices'
                WHEN primary_property_count=1 AND property_group NOT IN ('mixed','unknown')
                     AND contract_price>0 THEN 'single_primary_transaction_price'
                ELSE 'unavailable'
            END AS price_derivation
        FROM packages
    ),
    qualified AS (
        SELECT priced.*,
            array_remove(ARRAY[
                CASE WHEN contract_date IS NULL THEN 'missing_contract_date'
                     WHEN contract_date<DATE '1900-01-01' OR contract_date>current_date
                     THEN 'invalid_contract_date' END,
                CASE WHEN area IS NULL OR area<=0 THEN 'missing_or_invalid_area' END,
                CASE WHEN analytical_price IS NULL OR analytical_price<=0
                     THEN 'missing_or_invalid_price' END,
                CASE WHEN property_group IN ('mixed','unknown')
                     THEN 'non_comparable_package' END
            ],NULL) AS quality_reasons
        FROM priced
    )
    SELECT
        qualified.transaction_id,qualified.contract_date,qualified.dataset_year,
        qualified.property_group,qualified.building_part_count,
        qualified.land_parcel_count,qualified.primary_property_count,
        qualified.area,qualified.analytical_price,
        CASE WHEN qualified.area>0 AND qualified.analytical_price>0
             THEN qualified.analytical_price/qualified.area END,
        qualified.price_derivation,
        cardinality(qualified.quality_reasons)=0,qualified.quality_reasons,
        qualified.location,region.id,municipality.id,district.id,community.id,settlement.id
    FROM qualified
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='statistical_region' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) region ON true
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='municipality' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) municipality ON true
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='city_district' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) district ON true
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='local_community' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) community ON true
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='settlement' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) settlement ON true;

    GET DIAGNOSTICS imported_rows=ROW_COUNT;
    ANALYZE analytics.sale_facts;
    UPDATE analytics.refresh_runs
    SET status='succeeded',finished_at=clock_timestamp(),
        details=jsonb_build_object('rows',imported_rows)
    WHERE id=refresh_run_id;
    RETURN imported_rows;
END
$$;

COMMENT ON TABLE analytics.sale_facts IS
    'One quality-controlled analytical row per sale transaction, refreshed from immutable raw imports.';
COMMENT ON COLUMN analytics.sale_facts.price_derivation IS
    'Complete component sum, one-primary-property transaction total, or unavailable.';
