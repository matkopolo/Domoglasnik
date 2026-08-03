CREATE TABLE IF NOT EXISTS analytics.rental_space_type_groups (
    source_code integer PRIMARY KEY,
    property_group text NOT NULL CHECK (property_group IN (
        'house','apartment','room','garage','commercial','unknown'
    )),
    description text NOT NULL
);

INSERT INTO analytics.rental_space_type_groups(source_code,property_group,description)
VALUES
    (1,'house','Stanovanjska hiša'),
    (2,'apartment','Stanovanje'),
    (3,'garage','Parkirni prostor'),
    (4,'garage','Garaža'),
    (5,'commercial','Pisarniški prostori'),
    (6,'commercial','Prostori za poslovanje s strankami'),
    (7,'commercial','Prostori za zdravstveno dejavnost'),
    (8,'commercial','Trgovski ali storitveni lokal'),
    (9,'commercial','Gostinski lokal'),
    (10,'commercial','Prostori za šport, kulturo ali izobraževanje'),
    (11,'commercial','Industrijski prostori'),
    (12,'commercial','Turistični nastanitveni objekt'),
    (13,'commercial','Kmetijski objekt'),
    (14,'unknown','Tehnični ali pomožni prostori'),
    (15,'unknown','Drugo'),
    (16,'room','Stanovanjska soba ali sobe')
ON CONFLICT(source_code) DO UPDATE SET
    property_group=excluded.property_group,description=excluded.description;

CREATE OR REPLACE FUNCTION analytics.classify_rental_package(space_types integer[])
RETURNS text
LANGUAGE sql
STABLE
AS $$
WITH groups AS (
    SELECT DISTINCT mapping.property_group
    FROM unnest(coalesce(space_types,ARRAY[]::integer[])) source(source_code)
    LEFT JOIN analytics.rental_space_type_groups mapping USING(source_code)
    WHERE mapping.property_group IS NOT NULL AND mapping.property_group<>'unknown'
)
SELECT CASE count(*)
    WHEN 0 THEN 'unknown'
    WHEN 1 THEN min(property_group)
    ELSE 'mixed'
END
FROM groups
$$;

CREATE TABLE IF NOT EXISTS analytics.rental_facts (
    transaction_id bigint PRIMARY KEY
        REFERENCES rental_transactions(transaction_id) ON DELETE CASCADE,
    contract_date date,
    rental_start_date date,
    rental_end_date date,
    dataset_year integer NOT NULL,
    property_group text NOT NULL CHECK (property_group IN (
        'house','apartment','room','garage','commercial','mixed','unknown'
    )),
    space_count integer NOT NULL,
    area numeric(16,2),
    monthly_rent_reported numeric(16,2),
    monthly_rent_ex_vat numeric(16,2),
    monthly_rent_per_sqm numeric(18,4),
    rent_derivation text NOT NULL CHECK (rent_derivation IN (
        'component_rents','homogeneous_transaction_rent','unavailable'
    )),
    rental_term text NOT NULL CHECK (rental_term IN (
        'fixed','indefinite','unknown'
    )),
    duration_months integer,
    operating_costs_included boolean,
    rental_transaction_type integer,
    is_market_rent boolean NOT NULL,
    is_quality_eligible boolean NOT NULL,
    representative_eligible boolean NOT NULL,
    quality_reasons text[] NOT NULL DEFAULT ARRAY[]::text[],
    cadastral_municipality_code integer,
    building_number text,
    building_part_number text,
    construction_year integer,
    location geometry(Point,3794),
    statistical_region_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    municipality_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    city_district_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    local_community_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    settlement_id bigint REFERENCES analytics.geographic_units(id) ON DELETE SET NULL,
    refreshed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS rental_facts_group_date_idx
    ON analytics.rental_facts(property_group,contract_date);
CREATE INDEX IF NOT EXISTS rental_facts_municipality_date_idx
    ON analytics.rental_facts(municipality_id,contract_date);
CREATE INDEX IF NOT EXISTS rental_facts_region_date_idx
    ON analytics.rental_facts(statistical_region_id,contract_date);
CREATE INDEX IF NOT EXISTS rental_facts_building_idx
    ON analytics.rental_facts(cadastral_municipality_code,building_number,property_group);
CREATE INDEX IF NOT EXISTS rental_facts_location_gist_idx
    ON analytics.rental_facts USING gist(location);

CREATE OR REPLACE FUNCTION analytics.refresh_rental_facts()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    imported_rows bigint;
    refresh_run_id bigint;
BEGIN
    INSERT INTO analytics.refresh_runs(process,status)
    VALUES ('rental_facts','running') RETURNING id INTO refresh_run_id;
    TRUNCATE analytics.rental_facts CASCADE;

    INSERT INTO analytics.rental_facts (
        transaction_id,contract_date,rental_start_date,rental_end_date,dataset_year,
        property_group,space_count,area,monthly_rent_reported,monthly_rent_ex_vat,
        monthly_rent_per_sqm,rent_derivation,rental_term,duration_months,
        operating_costs_included,rental_transaction_type,is_market_rent,
        is_quality_eligible,representative_eligible,quality_reasons,
        cadastral_municipality_code,building_number,building_part_number,
        construction_year,location,statistical_region_id,municipality_id,
        city_district_id,local_community_id,settlement_id
    )
    WITH spaces AS (
        SELECT
            part.transaction_id,
            count(*)::integer AS space_count,
            count(*) FILTER(WHERE part.contractual_rent>0)::integer AS priced_count,
            sum(part.contractual_rent) FILTER(WHERE part.contractual_rent>0) AS component_rent,
            array_remove(array_agg(DISTINCT part.rented_spaces_type),NULL) AS source_types,
            sum(coalesce(part.rented_spaces_area,part.rented_usable_area))
                FILTER(WHERE coalesce(part.rented_spaces_area,part.rented_usable_area)>0) AS area,
            min(part.cadastral_municipality_code) AS cadastral_municipality_code,
            min(part.building_number) AS building_number,
            min(part.building_part_number) AS building_part_number,
            min(part.construction_year) AS construction_year,
            ST_Centroid(ST_Collect(part.location)
                FILTER(WHERE part.location IS NOT NULL)) AS location
        FROM rental_building_parts part
        GROUP BY part.transaction_id
    ),
    packages AS (
        SELECT
            transaction.transaction_id,transaction.contract_date,
            transaction.rental_start_date,transaction.rental_end_date,
            transaction.dataset_year,transaction.contract_rent,
            transaction.vat_included,transaction.vat_rate,
            transaction.rental_time,transaction.rental_duration,
            transaction.operating_costs_included,transaction.rental_transaction_type,
            analytics.classify_rental_package(spaces.source_types) AS property_group,
            coalesce(spaces.space_count,0) AS space_count,spaces.area,
            spaces.cadastral_municipality_code,spaces.building_number,
            spaces.building_part_number,spaces.construction_year,spaces.location,
            CASE
                WHEN spaces.space_count>0 AND spaces.priced_count=spaces.space_count
                     AND spaces.component_rent>0 THEN spaces.component_rent
                WHEN analytics.classify_rental_package(spaces.source_types)
                     NOT IN ('mixed','unknown') AND transaction.contract_rent>0
                    THEN transaction.contract_rent
            END AS reported_rent,
            CASE
                WHEN spaces.space_count>0 AND spaces.priced_count=spaces.space_count
                     AND spaces.component_rent>0 THEN 'component_rents'
                WHEN analytics.classify_rental_package(spaces.source_types)
                     NOT IN ('mixed','unknown') AND transaction.contract_rent>0
                    THEN 'homogeneous_transaction_rent'
                ELSE 'unavailable'
            END AS rent_derivation
        FROM rental_transactions transaction
        LEFT JOIN spaces USING(transaction_id)
    ),
    normalized AS (
        SELECT packages.*,
            CASE
                WHEN vat_included IS NOT TRUE THEN reported_rent
                WHEN vat_rate=1 THEN reported_rent/1.085
                WHEN vat_rate=2 THEN reported_rent/1.20
                WHEN vat_rate=3 THEN reported_rent/1.095
                WHEN vat_rate=4 THEN reported_rent/1.22
            END AS rent_ex_vat,
            CASE rental_time WHEN '1' THEN 'fixed'
                             WHEN '2' THEN 'indefinite'
                             ELSE 'unknown' END AS rental_term,
            CASE WHEN rental_time='1' AND rental_duration~'^[0-9]+$'
                 THEN rental_duration::integer END AS duration_months
        FROM packages
    ),
    qualified AS (
        SELECT normalized.*,
            array_remove(ARRAY[
                CASE WHEN contract_date IS NULL THEN 'missing_contract_date'
                     WHEN contract_date<DATE '1900-01-01' OR contract_date>current_date
                     THEN 'invalid_contract_date' END,
                CASE WHEN rental_transaction_type<>1 THEN 'non_market_rent' END,
                CASE WHEN property_group IN ('mixed','unknown')
                     THEN 'non_comparable_package' END,
                CASE WHEN area IS NULL OR area<=0 THEN 'missing_or_invalid_area' END,
                CASE WHEN rent_ex_vat IS NULL OR rent_ex_vat<=0
                     THEN 'missing_or_invalid_monthly_rent' END,
                CASE WHEN vat_included IS TRUE AND vat_rate NOT IN (1,2,3,4)
                     THEN 'unknown_vat_rate' END,
                CASE WHEN rental_term='fixed'
                          AND (duration_months IS NULL OR duration_months<6)
                     THEN 'invalid_fixed_term_duration' END,
                CASE
                    WHEN property_group='apartment' AND (area<10 OR area>300)
                        THEN 'implausible_area_for_property_group'
                    WHEN property_group='room' AND (area<5 OR area>100)
                        THEN 'implausible_area_for_property_group'
                    WHEN property_group='house' AND (area<20 OR area>2000)
                        THEN 'implausible_area_for_property_group'
                    WHEN property_group='garage' AND (area<2 OR area>200)
                        THEN 'implausible_area_for_property_group'
                    WHEN property_group='commercial' AND (area<5 OR area>50000)
                        THEN 'implausible_area_for_property_group' END,
                CASE
                    WHEN property_group='apartment'
                         AND (rent_ex_vat<10 OR rent_ex_vat>10000
                              OR rent_ex_vat/NULLIF(area,0)<1
                              OR rent_ex_vat/NULLIF(area,0)>50)
                        THEN 'implausible_rent_for_property_group'
                    WHEN property_group='room'
                         AND (rent_ex_vat<10 OR rent_ex_vat>5000
                              OR rent_ex_vat/NULLIF(area,0)<1
                              OR rent_ex_vat/NULLIF(area,0)>100)
                        THEN 'implausible_rent_for_property_group'
                    WHEN property_group IN ('house','garage','commercial')
                         AND (rent_ex_vat/NULLIF(area,0)<0.1
                              OR rent_ex_vat/NULLIF(area,0)>500)
                        THEN 'implausible_rent_for_property_group' END
            ],NULL) AS reasons
        FROM normalized
    )
    SELECT
        qualified.transaction_id,qualified.contract_date,
        qualified.rental_start_date,qualified.rental_end_date,qualified.dataset_year,
        qualified.property_group,qualified.space_count,qualified.area,
        qualified.reported_rent,qualified.rent_ex_vat,
        qualified.rent_ex_vat/NULLIF(qualified.area,0),qualified.rent_derivation,
        qualified.rental_term,qualified.duration_months,
        qualified.operating_costs_included,qualified.rental_transaction_type,
        qualified.rental_transaction_type=1,
        cardinality(qualified.reasons)=0,cardinality(qualified.reasons)=0,
        qualified.reasons,qualified.cadastral_municipality_code,
        qualified.building_number,qualified.building_part_number,
        qualified.construction_year,qualified.location,
        municipality.parent_id,municipality.id,
        district.id,community.id,settlement.id
    FROM qualified
    LEFT JOIN LATERAL (
        SELECT id,parent_id FROM analytics.geographic_units
        WHERE level='settlement' AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) settlement ON true
    LEFT JOIN LATERAL (
        SELECT id FROM analytics.geographic_units
        WHERE level='municipality' AND settlement.id IS NULL
          AND qualified.location IS NOT NULL
          AND ST_Covers(geometry_d96tm,qualified.location)
        ORDER BY ST_Area(geometry_d96tm),id LIMIT 1
    ) municipality_fb ON true
    LEFT JOIN analytics.geographic_units municipality
        ON municipality.id=coalesce(settlement.parent_id,municipality_fb.id)
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
    ) community ON true;

    GET DIAGNOSTICS imported_rows=ROW_COUNT;
    ANALYZE analytics.rental_facts;
    UPDATE analytics.refresh_runs
    SET status='succeeded',finished_at=clock_timestamp(),
        details=jsonb_build_object('rows',imported_rows,'ruleVersion','rental-facts-v1')
    WHERE id=refresh_run_id;
    RETURN imported_rows;
END
$$;

COMMENT ON TABLE analytics.rental_facts IS
    'One normalized monthly-rent fact per GURS rental agreement; raw records remain unchanged.';
