# Data pipeline worker

Detailed definitions for quality populations, robust cohort outliers,
duplicate signals, valuation-model selection, rolling temporal validation,
building/municipality robustness, split-conformal ranges, Isolation Forest,
and market indices are in
[../../docs/statistical-methodology.md](../../docs/statistical-methodology.md).

In particular, the methodology explains how many rows are eligible, how
training/calibration/validation years are formed, why models are national per
transaction kind and property group rather than per region, and where exact
per-run counts and selected parameters are persisted. Measured results from the
representative optimization run are in
[../../docs/model-optimization-report.md](../../docs/model-optimization-report.md).

The production-style pipeline runs inside Docker. The image contains Python
3.11, the GURS downloader/importer, psycopg, NumPy, SciPy, and scikit-learn.
Host Python is needed only for optional local development.

## Scheduled operation

Compose starts `pipeline-worker` with:

```text
real-estate-etl schedule --interval-days 14
```

It runs once immediately, writes its health marker only after the complete
source/geography/analytics/model chain succeeds, then waits 14 days. Failures
retry after one hour. A PostgreSQL session advisory lock prevents a scheduled
run, manual refresh, and reset from executing concurrently.

Each incremental run:

- reads successful national product-years from `source_files`;
- imports only missing BUY years from 2007 and RENT years from 2013;
- retrieves the current-year products again to detect revised archives;
- keeps valid ZIPs in the `pipeline_data` Docker volume;
- refreshes GURS RPE boundaries;
- refreshes all deterministic analytics in dependency order;
- trains and persists the versioned sale and rental models.

Configure the loop in the repository `.env`:

```dotenv
REFRESH_INTERVAL_DAYS=14
REFRESH_RETRY_HOURS=1
```

## Commands

Start the entire system and wait for the initial pipeline run:

```bash
docker compose up -d --build --wait
docker compose logs -f pipeline-worker
```

Run immediately:

```bash
docker compose run --rm pipeline-worker \
  refresh-system --output-dir /data/raw
```

Refresh only deterministic SQL analytics:

```bash
docker compose run --rm pipeline-worker refresh-analytics
```

Reset and rebuild imported/derived market data:

```bash
docker compose run --rm pipeline-worker \
  reset-and-repopulate --confirm-reset --output-dir /data/raw
```

Reset is intentionally explicit and destructive. It removes source-file
ledger rows, imported transactions/components, analytical facts, anomaly
signals, model runs/predictions, and index vintages. It preserves users,
sessions, preferences, classification tables, and cached ZIP archives.

Follow model training and validation output with:

```bash
docker compose logs -f pipeline-worker
```

For each sale/rental property group, model selection evaluates 20 candidate
configurations—10 Ridge, 6 Elastic Net, and 4 histogram-gradient-boosting—on
every valid expanding temporal fold. It then performs one final-holdout fit,
one building-first grouped-robustness fit, and one full-population production
fit. A group with 16 folds therefore performs approximately
`20 × 16 + 3 = 323` supervised fits. After all groups for sales or rentals are
finished, one separate Isolation Forest is trained for that complete
transaction-kind population. The detailed execution sequence, split rules,
fit-count formula, and persistence behavior are documented under “End-to-end
training execution and fit count” in
[../../docs/statistical-methodology.md](../../docs/statistical-methodology.md).

Each completed sale/rent model prints overall temporal-holdout metrics,
90% interval coverage, property-group diagnostics, comparison with a simple
training-median baseline, building-first grouped-holdout performance, and
warnings for materially low overall interval coverage. To rerun only model
training:

```bash
docker compose run --rm pipeline-worker refresh-models --workers 6
```

## Optional local development

```bash
python3 -m venv .venv
.venv/bin/pip install -e "apps/data-pipeline[dev,ml]"
cd apps/data-pipeline
../../.venv/bin/ruff check .
../../.venv/bin/pytest -q
```
