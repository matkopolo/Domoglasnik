ALTER TABLE analytics.model_runs
    ADD COLUMN IF NOT EXISTS selection_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS final_holdout_metrics jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN analytics.model_runs.selection_metadata IS
    'Candidate grids, temporal folds, selected configuration, grouped robustness, and selection rule; excludes final-holdout outcomes from selection.';
COMMENT ON COLUMN analytics.model_runs.final_holdout_metrics IS
    'One-time evaluation metrics for the latest complete calendar year, stored separately from cross-validation metrics.';
