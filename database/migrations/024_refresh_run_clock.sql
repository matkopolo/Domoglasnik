ALTER TABLE analytics.refresh_runs
    ALTER COLUMN started_at SET DEFAULT clock_timestamp();

COMMENT ON COLUMN analytics.refresh_runs.started_at IS
    'Wall-clock start of this individual refresh stage; uses clock_timestamp so stages in one transaction retain independent timings.';
