CREATE TABLE IF NOT EXISTS analytics.pipeline_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    model_bootstrap_completed_at timestamptz,
    model_bootstrap_version text
);

INSERT INTO analytics.pipeline_state(singleton)
VALUES(true)
ON CONFLICT(singleton) DO NOTHING;

UPDATE analytics.pipeline_state state
SET model_bootstrap_completed_at=existing.completed_at,
    model_bootstrap_version=existing.model_versions
FROM (
    SELECT max(created_at) completed_at,
           string_agg(model_version,',' ORDER BY transaction_kind) model_versions
    FROM (
        SELECT DISTINCT ON(transaction_kind)
               transaction_kind,created_at,model_version
        FROM analytics.model_runs
        ORDER BY transaction_kind,created_at DESC
    ) latest
    HAVING count(*)=2
) existing
WHERE state.singleton
  AND state.model_bootstrap_completed_at IS NULL;

COMMENT ON TABLE analytics.pipeline_state IS
    'Durable installation state. Market-data reset preserves this row; deleting the database volume does not.';
COMMENT ON COLUMN analytics.pipeline_state.model_bootstrap_completed_at IS
    'Set after both valuation populations train successfully; NULL requests one-time first-install training.';
