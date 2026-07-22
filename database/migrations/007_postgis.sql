CREATE EXTENSION IF NOT EXISTS postgis;

ALTER TABLE building_parts
    ADD COLUMN IF NOT EXISTS location geometry(Point, 3794)
    GENERATED ALWAYS AS (
        CASE WHEN centroid_e IS NOT NULL AND centroid_n IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(centroid_e::double precision, centroid_n::double precision), 3794)
        END
    ) STORED;

ALTER TABLE land_parcels
    ADD COLUMN IF NOT EXISTS location geometry(Point, 3794)
    GENERATED ALWAYS AS (
        CASE WHEN centroid_e IS NOT NULL AND centroid_n IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(centroid_e::double precision, centroid_n::double precision), 3794)
        END
    ) STORED;

ALTER TABLE rental_building_parts
    ADD COLUMN IF NOT EXISTS location geometry(Point, 3794)
    GENERATED ALWAYS AS (
        CASE WHEN centroid_e IS NOT NULL AND centroid_n IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(centroid_e::double precision, centroid_n::double precision), 3794)
        END
    ) STORED;

CREATE INDEX IF NOT EXISTS building_parts_location_gist_idx
    ON building_parts USING gist (location);
CREATE INDEX IF NOT EXISTS land_parcels_location_gist_idx
    ON land_parcels USING gist (location);
CREATE INDEX IF NOT EXISTS rental_building_parts_location_gist_idx
    ON rental_building_parts USING gist (location);
