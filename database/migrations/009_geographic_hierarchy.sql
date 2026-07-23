ALTER TABLE analytics.geographic_units
    ADD COLUMN IF NOT EXISTS parent_gurs_eid text;

ALTER TABLE analytics.geographic_units
    DROP CONSTRAINT IF EXISTS geographic_units_parent_id_fkey;
ALTER TABLE analytics.geographic_units
    ADD CONSTRAINT geographic_units_parent_id_fkey
    FOREIGN KEY (parent_id)
    REFERENCES analytics.geographic_units(id)
    ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS geographic_units_parent_gurs_eid_idx
    ON analytics.geographic_units (parent_gurs_eid);
