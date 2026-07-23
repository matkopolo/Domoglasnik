CREATE SCHEMA IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.geographic_units (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    level text NOT NULL CHECK (level IN (
        'country',
        'statistical_region',
        'municipality',
        'city_district',
        'local_community',
        'settlement'
    )),
    source_collection text NOT NULL,
    gurs_eid text NOT NULL,
    code integer,
    composite_code integer,
    name text NOT NULL,
    parent_id bigint REFERENCES analytics.geographic_units(id) ON DELETE CASCADE,
    parent_gurs_eid text,
    source_updated_at timestamptz,
    source_properties jsonb NOT NULL DEFAULT '{}'::jsonb,
    geometry_wgs84 geometry(MultiPolygon, 4326) NOT NULL,
    geometry_d96tm geometry(MultiPolygon, 3794)
        GENERATED ALWAYS AS (
            ST_Multi(ST_Transform(geometry_wgs84, 3794))
        ) STORED,
    imported_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_collection, gurs_eid)
);

CREATE INDEX IF NOT EXISTS geographic_units_level_idx
    ON analytics.geographic_units (level);
CREATE INDEX IF NOT EXISTS geographic_units_parent_idx
    ON analytics.geographic_units (parent_id);
CREATE INDEX IF NOT EXISTS geographic_units_name_idx
    ON analytics.geographic_units (lower(name));
CREATE INDEX IF NOT EXISTS geographic_units_wgs84_gist_idx
    ON analytics.geographic_units USING gist (geometry_wgs84);
CREATE INDEX IF NOT EXISTS geographic_units_d96tm_gist_idx
    ON analytics.geographic_units USING gist (geometry_d96tm);

CREATE TABLE IF NOT EXISTS analytics.refresh_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    process text NOT NULL,
    status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    source_version text,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    CHECK (
        (status = 'running' AND finished_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND finished_at IS NOT NULL)
    )
);

COMMENT ON SCHEMA analytics IS
    'Derived, quality-controlled data products consumed by the read-only API.';
COMMENT ON TABLE analytics.geographic_units IS
    'Versioned GURS RPE geographic units and PostGIS boundaries; populated by the data pipeline.';
COMMENT ON TABLE analytics.refresh_runs IS
    'Audit trail for analytics and boundary refresh jobs.';
