BEGIN;

ALTER TABLE building_parts DROP CONSTRAINT IF EXISTS building_parts_pkey;
ALTER TABLE building_parts ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;
ALTER TABLE building_parts ADD COLUMN IF NOT EXISTS source_row_number integer;
ALTER TABLE building_parts ALTER COLUMN building_number DROP NOT NULL;
ALTER TABLE building_parts ALTER COLUMN building_part_number DROP NOT NULL;
UPDATE building_parts SET source_row_number = id WHERE source_row_number IS NULL;
ALTER TABLE building_parts ALTER COLUMN source_row_number SET NOT NULL;
ALTER TABLE building_parts ADD PRIMARY KEY (id);
ALTER TABLE building_parts DROP CONSTRAINT IF EXISTS building_parts_source_file_id_source_row_number_key;
ALTER TABLE building_parts ADD UNIQUE (source_file_id, source_row_number);

ALTER TABLE land_parcels DROP CONSTRAINT IF EXISTS land_parcels_pkey;
ALTER TABLE land_parcels ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;
ALTER TABLE land_parcels ADD COLUMN IF NOT EXISTS source_row_number integer;
ALTER TABLE land_parcels ALTER COLUMN parcel_number DROP NOT NULL;
UPDATE land_parcels SET source_row_number = id WHERE source_row_number IS NULL;
ALTER TABLE land_parcels ALTER COLUMN source_row_number SET NOT NULL;
ALTER TABLE land_parcels ADD PRIMARY KEY (id);
ALTER TABLE land_parcels DROP CONSTRAINT IF EXISTS land_parcels_source_file_id_source_row_number_key;
ALTER TABLE land_parcels ADD UNIQUE (source_file_id, source_row_number);

COMMIT;
