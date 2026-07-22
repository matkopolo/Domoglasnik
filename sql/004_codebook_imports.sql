CREATE TABLE IF NOT EXISTS codebook_imports (
    checksum char(64) PRIMARY KEY,
    source_file_id bigint NOT NULL REFERENCES source_files(id),
    imported_at timestamptz NOT NULL DEFAULT now()
);
