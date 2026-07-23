CREATE TABLE IF NOT EXISTS analytics.model_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    model_version text NOT NULL,
    model_type text NOT NULL,
    feature_schema jsonb NOT NULL,
    training_rows integer NOT NULL,
    training_from date,
    training_to date,
    metrics jsonb NOT NULL,
    random_seed integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(transaction_kind,model_version)
);

CREATE TABLE IF NOT EXISTS analytics.valuation_predictions (
    transaction_kind text NOT NULL CHECK (transaction_kind IN ('sale','rent')),
    transaction_id bigint NOT NULL,
    model_run_id uuid NOT NULL REFERENCES analytics.model_runs(id),
    predicted_amount_per_sqm numeric(18,4) NOT NULL,
    lower_amount_per_sqm numeric(18,4) NOT NULL,
    upper_amount_per_sqm numeric(18,4) NOT NULL,
    actual_amount_per_sqm numeric(18,4) NOT NULL,
    log_residual numeric(18,8) NOT NULL,
    scored_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(transaction_kind,transaction_id)
);
CREATE INDEX IF NOT EXISTS valuation_predictions_model_idx
    ON analytics.valuation_predictions(model_run_id);

GRANT SELECT ON analytics.model_runs,analytics.valuation_predictions
    TO real_estate_api;

COMMENT ON TABLE analytics.model_runs IS
    'Versioned hedonic model metadata and out-of-fold evaluation metrics.';
COMMENT ON TABLE analytics.valuation_predictions IS
    'Transaction-level hedonic estimates and empirical prediction ranges.';
