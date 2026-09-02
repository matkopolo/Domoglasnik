ALTER TABLE analytics.valuation_predictions
    DROP CONSTRAINT IF EXISTS valuation_predictions_pkey;
ALTER TABLE analytics.valuation_predictions
    ADD PRIMARY KEY(transaction_kind,transaction_id,model_run_id);

CREATE INDEX IF NOT EXISTS valuation_predictions_lookup_idx
    ON analytics.valuation_predictions(transaction_kind,transaction_id,scored_at DESC);

CREATE TABLE IF NOT EXISTS analytics.model_publications (
    transaction_kind text PRIMARY KEY CHECK (transaction_kind IN ('sale','rent')),
    model_run_id uuid NOT NULL UNIQUE REFERENCES analytics.model_runs(id) ON DELETE CASCADE,
    activated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO analytics.model_publications(transaction_kind,model_run_id)
SELECT DISTINCT ON (prediction.transaction_kind)
       prediction.transaction_kind,prediction.model_run_id
FROM analytics.valuation_predictions prediction
JOIN analytics.model_runs run ON run.id=prediction.model_run_id
ORDER BY prediction.transaction_kind,run.created_at DESC,run.id DESC
ON CONFLICT(transaction_kind) DO NOTHING;

CREATE OR REPLACE VIEW analytics.current_valuation_predictions AS
SELECT prediction.*
FROM analytics.valuation_predictions prediction
JOIN analytics.model_publications publication
  ON publication.transaction_kind=prediction.transaction_kind
 AND publication.model_run_id=prediction.model_run_id;

CREATE TABLE IF NOT EXISTS analytics.analytical_releases (
    id uuid PRIMARY KEY,
    status text NOT NULL CHECK (status IN ('processing','published','failed')),
    stage text NOT NULL,
    source_sale_through date,
    source_rent_through date,
    analytics_sale_through date,
    analytics_rent_through date,
    model_versions text,
    started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    published_at timestamptz,
    failed_at timestamptz,
    failure_message text,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    CHECK ((status='published')=(published_at IS NOT NULL)),
    CHECK ((status='failed')=(failed_at IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS analytical_releases_one_processing_idx
    ON analytics.analytical_releases(status) WHERE status='processing';

CREATE TABLE IF NOT EXISTS analytics.analytical_publication (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    active_release_id uuid REFERENCES analytics.analytical_releases(id),
    pending_release_id uuid REFERENCES analytics.analytical_releases(id),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO analytics.analytical_publication(singleton) VALUES(true)
ON CONFLICT(singleton) DO NOTHING;

INSERT INTO analytics.analytical_releases(
    id,status,stage,source_sale_through,source_rent_through,
    analytics_sale_through,analytics_rent_through,model_versions,published_at
)
SELECT gen_random_uuid(),'published','published',
       (SELECT max(contract_date) FROM transactions),
       (SELECT max(contract_date) FROM rental_transactions),
       (SELECT max(contract_date) FROM analytics.sale_facts),
       (SELECT max(contract_date) FROM analytics.rental_facts),
       (SELECT model_bootstrap_version FROM analytics.pipeline_state WHERE singleton),
       clock_timestamp()
WHERE NOT EXISTS (SELECT 1 FROM analytics.analytical_releases);

UPDATE analytics.analytical_publication publication
SET active_release_id=(
        SELECT id FROM analytics.analytical_releases
        WHERE status='published' ORDER BY published_at DESC LIMIT 1
    ), updated_at=clock_timestamp()
WHERE singleton AND active_release_id IS NULL;

GRANT SELECT ON analytics.model_publications,
                analytics.current_valuation_predictions,
                analytics.analytical_releases,
                analytics.analytical_publication TO real_estate_api;

COMMENT ON TABLE analytics.model_publications IS
    'Atomic pointer to the active immutable valuation-model generation for each transaction kind.';
COMMENT ON VIEW analytics.current_valuation_predictions IS
    'Predictions from the active model generation; pending model training remains invisible.';
COMMENT ON TABLE analytics.analytical_releases IS
    'Publication history for coherent facts, quality analytics, indices, and valuation models.';
COMMENT ON TABLE analytics.analytical_publication IS
    'Singleton pointer exposing the active release and any background release in progress.';
