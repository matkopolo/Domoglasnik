BEGIN;

ALTER TABLE building_parts ALTER COLUMN cadastral_municipality_code DROP NOT NULL;
ALTER TABLE land_parcels ALTER COLUMN cadastral_municipality_code DROP NOT NULL;

COMMIT;
