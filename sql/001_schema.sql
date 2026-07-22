CREATE TABLE IF NOT EXISTS municipalities (
    code integer PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE IF NOT EXISTS cadastral_municipalities (
    code integer PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE IF NOT EXISTS source_files (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    file_name text NOT NULL,
    municipality_code integer REFERENCES municipalities(code),
    dataset_year integer,
    dataset_type text NOT NULL DEFAULT 'sales' CHECK (dataset_type IN ('sales', 'rent')),
    dataset_scope text NOT NULL DEFAULT 'municipality' CHECK (dataset_scope IN ('municipality', 'national')),
    sha256 char(64) NOT NULL UNIQUE,
    downloaded_at timestamptz,
    imported_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS codebook_values (
    codebook_id integer NOT NULL,
    codebook_name text NOT NULL,
    numeric_value integer NOT NULL,
    description text NOT NULL,
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    PRIMARY KEY (codebook_id, numeric_value)
);

CREATE TABLE IF NOT EXISTS codebook_imports (
    checksum char(64) PRIMARY KEY,
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    imported_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    transaction_id bigint PRIMARY KEY,
    transaction_type integer,
    effective_date date,
    contract_date date,
    contract_price numeric(16,2),
    vat_included boolean,
    vat_rate numeric(8,4),
    lease_expiry_date date,
    lease_termination_date date,
    notes text,
    agency_brokered boolean,
    last_changed_date date,
    last_effective_date date,
    deed_type integer,
    marketability integer,
    dataset_year integer NOT NULL,
    municipality_code integer REFERENCES municipalities(code),
    source_file_id bigint NOT NULL REFERENCES source_files(id)
);

CREATE TABLE IF NOT EXISTS building_parts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id bigint NOT NULL REFERENCES transactions(transaction_id) ON DELETE CASCADE,
    cadastral_municipality_code integer REFERENCES cadastral_municipalities(code),
    building_number text,
    building_part_number text,
    cadastral_municipality_name text,
    municipality_name text,
    parcel_number_for_geolocation text,
    internal_building_part_label text,
    recorded_status integer,
    settlement text,
    street text,
    house_number text,
    house_number_suffix text,
    apartment_or_business_unit_number text,
    building_part_type integer,
    construction_year integer,
    building_completed boolean,
    construction_phase integer,
    new_construction boolean,
    sold_area numeric(14,2),
    sold_share text,
    sold_building_part_area numeric(14,2),
    sold_usable_area numeric(14,2),
    floor text,
    outdoor_parking_spaces integer,
    atrium boolean,
    atrium_area numeric(14,2),
    property_notes text,
    actual_use text,
    position_in_building text,
    room_count numeric(8,2),
    building_part_area numeric(14,2),
    usable_area numeric(14,2),
    rooms text,
    contractual_price numeric(16,2),
    vat_rate numeric(8,4),
    centroid_e numeric(12,2),
    centroid_n numeric(12,2),
    dataset_year integer NOT NULL,
    municipality_code integer REFERENCES municipalities(code),
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    source_row_number integer NOT NULL,
    UNIQUE (source_file_id, source_row_number)
);

CREATE TABLE IF NOT EXISTS land_parcels (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id bigint NOT NULL REFERENCES transactions(transaction_id) ON DELETE CASCADE,
    cadastral_municipality_code integer REFERENCES cadastral_municipalities(code),
    parcel_number text,
    cadastral_municipality_name text,
    municipality_name text,
    land_type integer,
    permanent_crop_type integer,
    permanent_crop_age integer,
    sold_share text,
    property_notes text,
    parcel_area numeric(14,2),
    contractual_price numeric(16,2),
    vat_rate numeric(8,4),
    centroid_e numeric(12,2),
    centroid_n numeric(12,2),
    dataset_year integer NOT NULL,
    municipality_code integer REFERENCES municipalities(code),
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    source_row_number integer NOT NULL,
    UNIQUE (source_file_id, source_row_number)
);

CREATE INDEX IF NOT EXISTS transactions_municipality_year_idx
    ON transactions (municipality_code, dataset_year);
CREATE INDEX IF NOT EXISTS transactions_contract_date_idx ON transactions (contract_date);
CREATE INDEX IF NOT EXISTS building_parts_location_idx ON building_parts (municipality_code, cadastral_municipality_code);
CREATE INDEX IF NOT EXISTS land_parcels_location_idx ON land_parcels (municipality_code, cadastral_municipality_code);
CREATE INDEX IF NOT EXISTS building_parts_transaction_idx ON building_parts (transaction_id);
CREATE INDEX IF NOT EXISTS land_parcels_transaction_idx ON land_parcels (transaction_id);

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
