CREATE TABLE IF NOT EXISTS analytics.building_part_type_groups (
    source_code integer PRIMARY KEY,
    property_group text NOT NULL CHECK (property_group IN (
        'house', 'apartment', 'garage', 'commercial', 'unknown'
    )),
    is_primary boolean NOT NULL,
    description text NOT NULL
);

INSERT INTO analytics.building_part_type_groups
    (source_code, property_group, is_primary, description)
VALUES
    (1, 'house', true, 'Stanovanjska hiša'),
    (2, 'apartment', true, 'Stanovanje'),
    (3, 'garage', false, 'Parkirni prostor'),
    (4, 'garage', false, 'Garaža'),
    (5, 'commercial', true, 'Pisarniški prostori'),
    (6, 'commercial', true, 'Prostori za poslovanje s strankami'),
    (7, 'commercial', true, 'Prostori za zdravstveno dejavnost'),
    (8, 'commercial', true, 'Trgovski ali storitveni lokal'),
    (9, 'commercial', true, 'Gostinski lokal'),
    (10, 'commercial', true, 'Prostori za šport, kulturo ali izobraževanje'),
    (11, 'commercial', true, 'Industrijski prostori'),
    (12, 'commercial', true, 'Turistični nastanitveni objekt'),
    (13, 'commercial', true, 'Kmetijski objekt'),
    (14, 'unknown', false, 'Tehnični ali pomožni prostori'),
    (15, 'unknown', false, 'Drugo')
ON CONFLICT (source_code) DO UPDATE SET
    property_group = EXCLUDED.property_group,
    is_primary = EXCLUDED.is_primary,
    description = EXCLUDED.description;

CREATE OR REPLACE FUNCTION analytics.classify_sale_package(
    building_part_types integer[],
    land_types integer[]
) RETURNS text
LANGUAGE sql
STABLE
AS $$
WITH building_groups AS (
    SELECT DISTINCT mapping.property_group, mapping.is_primary
    FROM unnest(coalesce(building_part_types, ARRAY[]::integer[])) AS source(source_code)
    LEFT JOIN analytics.building_part_type_groups mapping USING (source_code)
),
primary_groups AS (
    SELECT property_group
    FROM building_groups
    WHERE is_primary AND property_group <> 'unknown'
),
summary AS (
    SELECT
        (SELECT count(*) FROM primary_groups) AS primary_group_count,
        (SELECT min(property_group) FROM primary_groups) AS primary_group,
        EXISTS (
            SELECT 1 FROM building_groups
            WHERE property_group = 'garage'
        ) AS has_garage,
        cardinality(coalesce(land_types, ARRAY[]::integer[])) > 0 AS has_land,
        cardinality(coalesce(building_part_types, ARRAY[]::integer[])) > 0 AS has_building
)
SELECT CASE
    WHEN primary_group_count > 1 THEN 'mixed'
    WHEN primary_group_count = 1 THEN primary_group
    WHEN has_garage THEN 'garage'
    WHEN has_land THEN 'land'
    WHEN has_building THEN 'unknown'
    ELSE 'unknown'
END
FROM summary
$$;

COMMENT ON TABLE analytics.building_part_type_groups IS
    'Versioned analytical mapping from GURS building-part codes to stable API property groups.';
COMMENT ON FUNCTION analytics.classify_sale_package(integer[], integer[]) IS
    'Classifies an entire sale package. Auxiliary garage and land components do not turn one primary property group into mixed.';
