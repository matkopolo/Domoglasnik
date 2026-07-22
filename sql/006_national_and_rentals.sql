BEGIN;

ALTER TABLE source_files ADD COLUMN IF NOT EXISTS dataset_type text NOT NULL DEFAULT 'sales';
ALTER TABLE source_files ADD COLUMN IF NOT EXISTS dataset_scope text NOT NULL DEFAULT 'municipality';

ALTER TABLE transactions ALTER COLUMN municipality_code DROP NOT NULL;
ALTER TABLE building_parts ALTER COLUMN municipality_code DROP NOT NULL;
ALTER TABLE land_parcels ALTER COLUMN municipality_code DROP NOT NULL;

CREATE TABLE IF NOT EXISTS rental_transactions (
    transaction_id bigint PRIMARY KEY,
    rental_transaction_type integer,
    effective_date date,
    contract_date date,
    contract_rent numeric(16,2),
    operating_costs_included boolean,
    vat_included boolean,
    vat_rate numeric(8,4),
    rental_start_date date,
    rental_end_date date,
    rental_time text,
    rental_duration text,
    closure_or_early_termination_date date,
    notes text,
    agency_brokered boolean,
    last_changed_date date,
    last_effective_date date,
    deed_type integer,
    marketability integer,
    dataset_year integer NOT NULL,
    source_file_id bigint NOT NULL REFERENCES source_files(id)
);

CREATE TABLE IF NOT EXISTS rental_building_parts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id bigint NOT NULL REFERENCES rental_transactions(transaction_id) ON DELETE CASCADE,
    cadastral_municipality_code integer REFERENCES cadastral_municipalities(code),
    cadastral_municipality_name text,
    municipality_name text,
    building_number text,
    building_part_number text,
    internal_rented_spaces_label text,
    settlement text,
    street text,
    house_number text,
    house_number_suffix text,
    apartment_or_business_unit_number text,
    rented_spaces_type integer,
    furnishing integer,
    microlocation integer,
    storefront boolean,
    shopping_center boolean,
    rented_spaces_area numeric(14,2),
    rented_area_equals_building_part boolean,
    contractual_rent numeric(16,2),
    rented_spaces_notes text,
    construction_year integer,
    actual_use text,
    position_in_building text,
    room_count numeric(8,2),
    building_part_area numeric(14,2),
    usable_building_part_area numeric(14,2),
    rooms text,
    rented_usable_area numeric(14,2),
    hospitality_garden boolean,
    garden_included_in_rent boolean,
    hospitality_garden_area numeric(14,2),
    centroid_e numeric(12,2),
    centroid_n numeric(12,2),
    dataset_year integer NOT NULL,
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    source_row_number integer NOT NULL,
    UNIQUE (source_file_id, source_row_number)
);

CREATE INDEX IF NOT EXISTS rental_transactions_year_idx ON rental_transactions (dataset_year);
CREATE INDEX IF NOT EXISTS rental_transactions_contract_date_idx ON rental_transactions (contract_date);
CREATE INDEX IF NOT EXISTS rental_building_parts_transaction_idx ON rental_building_parts (transaction_id);
CREATE INDEX IF NOT EXISTS rental_building_parts_location_idx
    ON rental_building_parts (municipality_name, cadastral_municipality_code);

COMMIT;
