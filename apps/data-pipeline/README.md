# Data pipeline worker

Detailed definitions for quality populations, robust cohort outliers,
duplicate signals, hedonic Ridge models, temporal/spatial validation,
split-conformal ranges, Isolation Forest, and market indices are in
[../../docs/statistical-methodology.md](../../docs/statistical-methodology.md).

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

Each completed sale/rent model prints overall temporal-holdout metrics,
90% interval coverage, property-group diagnostics, comparison with a simple
local-median baseline, municipality-holdout performance, and warnings for low
coverage or negative R². To rerun only model training:

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
