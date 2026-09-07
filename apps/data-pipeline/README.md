# Data pipeline worker

Production deployment uses [K3s/CloudNativePG](../../docs/k3s-deployment.md).
`MODEL_REFRESH_POLICY=manual` disables automatic model training during scheduled
refresh and initial import; use an explicit `refresh-models` Job. The default
`auto` preserves Compose's initial/new-archive training behavior. Manual analytics
and model commands share the scheduler's PostgreSQL advisory lock.

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

It runs once immediately, writes its health marker after the required
source/geography/analytics chain succeeds, then waits 14 days. A new database
also performs the initial model bootstrap before becoming healthy. Failures
retry after one hour. A PostgreSQL session advisory lock prevents a scheduled
run, manual refresh, and reset from executing concurrently.

Each incremental run:

- reads successful national product-years from `source_files`;
- imports only missing BUY years from 2007 and RENT years from 2013;
- retrieves the current-year products again to detect revised archives;
- keeps valid ZIPs in the `pipeline_data` Docker volume;
- refreshes GURS RPE boundaries;
- refreshes all deterministic analytics in dependency order inside one
  publication transaction;
- retrains sale and rental models when an archive checksum changed, while
  unchanged current-year downloads retain the published model;
- keeps the previous facts, outliers, indices, and model predictions visible
  until the complete pending release commits.

`analytics.analytical_publication` identifies the active and pending releases.
During processing, `/api/v1/data-status` exposes both the published analytical
cutoff and the newer GURS source cutoff. A failed refresh rolls back derived
changes and leaves the active release untouched.

Before every SQL analytics function starts, the worker prints a short statement
of what it derives and whether the operation is non-destructive. In particular,
outlier stages mark statistical deviations and update representative-sample
eligibility; they never delete the underlying GURS transaction.

Model refreshes likewise announce the sale and rental learning populations
before fitting starts: the target, per-property-group separation, duplicate and
strict-land eligibility, and leakage-safe time-ordered selection/calibration.
Every stage also logs the exact schema-qualified database tables it affects and
explains whether it rebuilds rows, inserts a version, or updates specific
eligibility, outlier, duplicate, or publication fields. Shared audit
bookkeeping such as `analytics.refresh_runs` is intentionally omitted from
these short destination lists.

The complete numbered analytics plan is logged before its first SQL function
starts and is the actual sequential execution order. The model plan is also
printed up front: sale completes before rent starts. It separately discloses
bounded property-group parallelism inside the currently active sale or rental
population so the log never implies that all model fits are sequential.

Configure the loop in the repository `.env`:

```dotenv
REFRESH_INTERVAL_DAYS=14
REFRESH_RETRY_HOURS=1
LOG_LEVEL=INFO
```

Operational messages use Python logging on stdout, so they remain visible in
`docker compose logs`. `INFO` includes plans, progress, database effects, and
completion summaries. Use `LOG_LEVEL=WARNING` for quieter scheduled operation
or `LOG_LEVEL=DEBUG` when additional debug messages are available. Routine
`httpx` request summaries—including signed download URLs—are debug-only and do
not appear at the default `INFO` level. CLI reports that are explicit command
results remain plain terminal output.

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

Long-running SQL analytics stages print a heartbeat every 15 seconds with the
stage name and elapsed time, followed by the exact affected-row count when the
atomic PostgreSQL statement completes. Set
`ANALYTICS_PROGRESS_INTERVAL_SECONDS` on the worker to use a different
interval. PostgreSQL does not expose a reliable intermediate inserted-row
count for these set-based transactional refreshes, so the heartbeat does not
claim an estimated percentage.

Reset and rebuild imported/derived market data:

```bash
docker compose run --rm pipeline-worker \
  reset-and-repopulate --confirm-reset --output-dir /data/raw
```

Reset is intentionally explicit and destructive. It removes source-file
ledger rows, imported transactions/components, analytical facts, anomaly
signals, model runs/predictions, and index vintages. It preserves users,
sessions, preferences, classification tables, the completed-bootstrap marker,
and cached ZIP archives. Consequently, reset/repopulate does not retrain the
models; run `refresh-models` explicitly afterward if valuation outputs are
required.

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

`--workers` controls bounded property-group concurrency during model
selection and Isolation Forest parallelism. Independent property groups run in
parallel, while database persistence remains single-threaded and transactional.
The Docker worker pins native BLAS/OpenMP pools to one thread per fit to avoid
nested oversubscription. For example:

```bash
docker compose run --rm pipeline-worker refresh-models --workers 6
```

Candidate models share only fold-local transformed feature matrices. Imputation,
scaling, category-frequency decisions, and one-hot encoding are fitted once per
temporal fold instead of once per candidate, while remaining isolated from all
later folds and the final holdout. This is an accuracy-neutral runtime
optimization; it does not reduce candidates, years, rows, or validation work.
Estimator fit time, shared fold-preprocessing time, and total optimization wall
time are persisted separately, avoiding duplicated runtime accounting.

Progress output reports the transaction kind, property-group position,
candidate position, progress at approximately 10% intervals, elapsed time, and
rolling ETA. A typical sequence is:

```text
[models] transaction kind 1/2 starting: sale
[sale model] property groups=5; parallel workers=5; database persistence remains single-threaded
[sale model · apartment · group 1/5] fit plan: 20 candidates × 16 folds = 320 candidate fits, plus holdout, grouped, and production stages
[sale model · apartment · group 1/5] progress 96/320 (30.0%): candidate 6/20, fold 16/16; elapsed=..., ETA=...
[sale model] group completion 3/5 (60.0%); elapsed=...; ETA=...
[models] transaction kind completion 1/2 (50.0%); elapsed=...; ETA=...
```

Each property group's ETA is based on its completed candidate fits; group and
transaction-kind ETAs are rolling estimates and may change when differently
sized populations or estimator families complete.

Set `MODEL_DETAILED_PROGRESS=true` when diagnosing model training to restore a
line for every temporal-fold fit. Concise milestone logging is the default
because several property groups write progress concurrently.

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

Database integration tests require `TEST_DATABASE_URL` to point to a disposable
PostGIS database with **all** `database/migrations/*.sql` files applied in order
before running pytest. In particular, publication tests require migration 036.
Without `TEST_DATABASE_URL`, database integration tests are skipped; a passing
unit-only run does not verify them. CI migrates its fresh database before the
Python tests and then runs the Go tests against that schema. For changes to
database setup or publication, verify this fresh-database sequence before
pushing; tests against an already populated development database can miss
missing migrations or incorrect setup order.
