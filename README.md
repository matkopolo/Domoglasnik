# Slovenian real-estate analytics monorepo

This project downloads national yearly ZIP archives from GURS JGP, preserves the raw files, and imports their CSV contents into PostgreSQL. It loads both purchase transactions (BUY/KPP, 2007 onward) and rental transactions (RENT/NP, 2013 onward). Imports are idempotent: reimporting a dataset/year replaces that database snapshot instead of duplicating it.

## Project structure

- `apps/data-pipeline/`: Python downloader, transformer, PostgreSQL loader, and tests
- `apps/real-estate-api/`: Go analytics API, OpenAPI contract, and tests
- `apps/web/`: Next.js real-estate market dashboard and map
- `database/migrations/`: shared PostgreSQL schema and incremental migrations
- `database/documentation/`: shared data-model documentation
- `docs/database.md`: database architecture, lifecycle, security, and operations
- `docs/frontend.md`: implemented frontend architecture and user behavior
- `docs/real-estate-api.md`: API-service architecture and contract overview
- `docs/statistical-methodology.md`: statistical populations, equations, models, validation, and limitations
- `docs/development-guidelines.md`: Go/Python conventions and test-layout policy
- `infrastructure/docker/`: service-specific Docker assets
- `data/raw/`: local downloaded archives, excluded from Git
- `Makefile`: root commands shared by all developers

## Run the complete system

Docker Desktop (or Docker Engine with Compose) is the only requirement for the
normal workflow on Windows, macOS, and Linux:

```bash
docker compose up -d --build --wait
```

When updating a checkout that previously used the Compose service name `api`,
remove that obsolete container once before starting the renamed service:

```bash
docker compose down --remove-orphans
docker compose up -d --build --wait
```

Named database and pipeline volumes are retained by these commands.

On the first run, `pipeline-worker` downloads and imports the national GURS
history, synchronizes geographic boundaries, refreshes analytics, and builds
the valuation/anomaly models. The API waits for that first successful refresh
before becoming ready, then the web application starts at
<http://localhost:3000>. Follow progress with:

```bash
docker compose logs -f pipeline-worker
```

PostgreSQL data and downloaded ZIP archives live in named Docker volumes and
survive `docker compose down` and application rebuilds.

### Rebuild or start completely fresh

To rebuild all four services—PostGIS, pipeline worker, real-estate API, and
web—while preserving imported data and downloaded archives:

```bash
docker compose down --remove-orphans
docker compose up -d --build --wait
docker compose logs -f pipeline-worker
```

For a genuinely empty installation, including an empty database and no cached
GURS archives, remove the named volumes as well. This permanently deletes
market data, users, preferences, model history, and cached downloads:

```bash
docker compose down --volumes --remove-orphans
docker compose up -d --build --wait
docker compose logs -f pipeline-worker
```

The first clean run downloads and imports GURS data, synchronizes geography,
refreshes deterministic analytics, trains models, starts `real-estate-api`,
and finally starts the frontend. Keep the log command open until the worker
prints validation reports for both sale and rent models.

## Visual Studio Code

Open [real-estate.code-workspace](real-estate.code-workspace) in VS Code instead of opening an individual application folder:

```bash
code real-estate.code-workspace
```

The workspace keeps the complete monorepo visible as one project and recommends the Python/Pylance/Ruff, Go, ESLint/Prettier, Container Tools, PostgreSQL, Makefile, and YAML extensions. Select **Install Workspace Recommended Extensions** when prompted.

Shared configuration in `.vscode/` provides:

- automatic Python environment discovery from `.venv` and pytest discovery from `apps/data-pipeline/tests`;
- Ruff formatting for Python, Go formatting/import organization, and Prettier for JavaScript/TypeScript/React;
- ignored generated folders for cleaner search and file watching without hiding the source `data` directory;
- tasks for starting/stopping the database, linting, testing, and running the verbose import;
- a **Python: ETL CLI** debugger configuration that can be edited to target another CLI command.

Run a task with **Terminal → Run Task**, run tests from VS Code's Testing panel, or press **F5** to debug the Go API or Python pipeline. The task list also starts, tests, lints, and tails logs for the web dashboard.

Inside the data pipeline:

- `api.py`: `JGPClient`, pooled HTTP connections, atomic ZIP downloads, and retry policy
- `importer.py`: streaming ZIP/CSV orchestration and transactional PostgreSQL `COPY`
- `conversion.py`: strict typed source conversion with row-level diagnostics
- `pipeline.py`: reference synchronization and bounded concurrent download orchestration
- `geography.py`: official GURS RPE boundary ingestion
- `model_features.py` / `model_validation.py`: tested model construction and validation policy
- `modeling.py`: offline model persistence and refresh orchestration
- `settings.py`: immutable API/product metadata and machine defaults
- `models.py`: typed results passed between download, parsing, and loading stages
- `cli.py`: argument parsing and command dispatch
- `application.py`: compatibility imports for older callers; new code uses the focused modules above
- `pyproject.toml`: installable package metadata, console command, dependencies, and Ruff rules

The PostgreSQL tables remain in `database/migrations`, not Python. CSV mappings are intentionally kept as one declarative registry rather than one file per table: they are source-to-destination translation rules, and keeping BUY and RENT mappings together makes schema drift comparisons and validation easier. If the number of source products grows substantially, that registry can be moved intact into `mappings.py` without changing the importer.

## Library choices

- **[HTTPX](https://www.python-httpx.org/)** provides connection pooling, streaming downloads, and explicit timeout/status handling.
- **[Tenacity](https://tenacity.readthedocs.io/en/stable/)** provides bounded exponential retry behavior for transient transport and server failures.
- **psycopg 3** streams rows through PostgreSQL `COPY`; ZIP members and
  codebooks are processed as streams or bounded batches. SQLAlchemy and pandas
  are intentionally not used because they would add memory and abstraction
  overhead to the bulk-loading path.
- **pytest** covers conversion, retry, resume, historical edge cases, and isolated PostgreSQL integration.
- **[Ruff](https://docs.astral.sh/ruff/)** enforces imports, correctness rules, modernization, and style locally and in GitHub Actions.

The package installs a `real-estate-etl` executable, so developers do not need to manage `PYTHONPATH` manually.

Repository-wide Go/Python conventions and the reason Go tests remain adjacent
to packages while Python tests live in a separate tree are documented in
[docs/development-guidelines.md](docs/development-guidelines.md).

The complete column-level reference is in [database/documentation/data-dictionary.md](database/documentation/data-dictionary.md). It documents source CSV names, English meanings, PostgreSQL types, units, nullability, codebooks, coordinates, relationships, and analysis cautions.

The current system references are
[database architecture](docs/database.md),
[frontend architecture](docs/frontend.md),
[real-estate API](docs/real-estate-api.md), and
[statistical methodology](docs/statistical-methodology.md). They describe the
implemented system rather than historical phase plans.

## Start PostgreSQL with PostGIS

```bash
make db-up
```

The default connection is `postgresql://real_estate:real_estate@localhost:5432/real_estate`. Copy `.env.example` to `.env` or export `DATABASE_URL` if you change it.

The database image is built from the official multi-architecture PostgreSQL 16 image with its matching PostGIS package, so it runs natively on Intel/AMD64 and Apple Silicon/ARM64 machines. `make db-up` builds and starts or upgrades the container, waits until it is healthy, and applies the idempotent spatial migration. The named Docker volume is preserved when the container image changes, so existing imported data does not need to be downloaded again.

The `building_parts`, `land_parcels`, and `rental_building_parts` tables expose a generated `location geometry(Point,3794)` column backed by a GiST spatial index. It is automatically derived from the original `centroid_e` and `centroid_n` values; the importer does not need to populate it.

To apply only the spatial migration to an already-running PostGIS container:

```bash
make db-migrate
```

Return longitude and latitude for a web map with:

```sql
SELECT
    id,
    ST_X(ST_Transform(location, 4326)) AS longitude,
    ST_Y(ST_Transform(location, 4326)) AS latitude
FROM building_parts
WHERE location IS NOT NULL;
```

The point is the source property's centroid, not necessarily an entrance or postal-address coordinate. A transaction can also reference multiple property rows and therefore multiple map points.

### Development memory budget

PostgreSQL/PostGIS remains the source of truth and analytical engine; the
dataset is never copied wholesale into Go memory. Docker Compose applies
development ceilings of 4 GB for PostgreSQL, 3 GB for the offline pipeline
worker, 512 MB for the Go API, and 768 MB for the web application. These are
maximums, not reserved memory.

Allocate 8–12 GB to Docker Desktop for a full model refresh alongside
PostgreSQL and image builds. The API uses bounded SQL queries, pagination, map
bounding boxes, and precomputed database aggregates.

## Start the Go analytics API

The API uses the existing PostgreSQL/PostGIS database and keeps derived API data in the separate `analytics` schema. It does not need or maintain a duplicate database.

```bash
make api-up
curl http://localhost:8080/health/ready
curl http://localhost:8080/api/v1/meta
```

Refresh the official GURS RPE statistical-region, municipality, city-district, local-community, and settlement hierarchy with:

```bash
docker compose run --rm pipeline-worker sync-geographies
```

The scheduled/full worker already performs downstream refreshes. For targeted
development, rebuild only the persisted analytics or models with:

```bash
make analytics-refresh
make models-refresh WORKERS=6
```

`analytics-refresh` rebuilds deterministic facts, strict land readiness,
robust outliers, missingness profiles, exact/near-duplicate signals, and the
matched-pair median index. `models-refresh` separately evaluates hedonic models
on the latest complete year and municipality holdouts, calibrates prediction
ranges on the preceding year, trains Isolation Forest models, and appends a
new hedonic-index vintage.

Model refresh logs report temporal holdout MAE, median absolute percentage
error, log RMSE/R², observed coverage of the 90% prediction interval,
property-group performance against a local-median baseline, and performance
on held-out municipalities. They deliberately do not print one generic
“accuracy” number because this is regression, not classification. Full
definitions and interpretation are in
[docs/statistical-methodology.md](docs/statistical-methodology.md).

For `propertyGroup=land`, API statistics and maps default to contracts whose
parcels are exclusively verified-buildable ETN codes 1 and 2. `landScope=broad`
opts into codes 1–3 and `landScope=all` explicitly requests every retained land
class. Raw and analytical land transactions are never deleted.

The initial analytical refresh takes several minutes; indexed API queries then
complete in milliseconds. The API role cannot mutate imported transactions or
run refreshes. It has narrowly scoped writes only for users, sessions,
preferences, and authenticated anomaly review decisions.

The full local contract and developer instructions are in [apps/real-estate-api/README.md](apps/real-estate-api/README.md). The API can be built and tested through Docker even when Go is not installed on the host:

```bash
make api-vet
make api-test
make api-build
```

Root `make lint` and `make test` run checks for the Python pipeline, Go API,
and TypeScript frontend. The root `package.json` is a deployment entrypoint
for monorepo-aware hosting; Docker, development dependencies, and
application-specific npm commands remain isolated in `apps/web`.

`make web-typecheck` covers production and test TypeScript.
`make web-generate-api-types` regenerates the checked-in transport schema from
OpenAPI, and `make web-contract-drift` fails if it has drifted.

## Automated data-pipeline worker

`pipeline-worker` contains Python and all ETL/model dependencies. It runs a
complete refresh immediately and then every **14 days**:

1. acquire a PostgreSQL advisory lock so scheduled and manual runs cannot
   overlap;
2. compare national BUY/RENT product-years with `source_files`;
3. download/import missing years and re-check the current year for revisions;
4. synchronize current GURS RPE geography;
5. refresh facts, strict-land populations, outliers, anomaly signals,
   duplicates, missingness profiles, and matched-pair indices;
6. refresh versioned hedonic models, intervals, Isolation Forest scores, and
   hedonic indices;
7. mark the worker healthy only after a complete successful run.

Failures retain the last committed database state and retry after one hour.
Override the interval or retry delay in `.env`:

```dotenv
REFRESH_INTERVAL_DAYS=14
REFRESH_RETRY_HOURS=1
```

Run the workflow immediately:

```bash
docker compose run --rm pipeline-worker refresh-system --output-dir /data/raw
```

Deliberately delete imported and derived market data and rebuild it:

```bash
docker compose run --rm pipeline-worker reset-and-repopulate \
  --confirm-reset --output-dir /data/raw
```

The reset preserves application users and saved preferences. The confirmation
flag is mandatory. Cached valid archives are reused, while the current year is
retrieved again. Equivalent convenience commands are `make start`,
`make refresh-system`, and `make reset-and-repopulate`.

## Start the web dashboard

The frontend, **Analitika nepremičnin**, is a responsive Next.js
dashboard with a real interactive MapLibre street/satellite map,
geography-aware Slovenia/region/municipality/local-area summaries, exact GURS
region boundaries, sale/rental switching, geographic and property filters,
quality-aware summary metrics, time series, and distributions:

```bash
make web-up
open http://localhost:3000
```

The browser calls the Go API at `http://localhost:8080`. When an API is not
reachable—such as the standalone hosted design preview—the interface clearly
labels and uses representative demo data. Configure a deployed API with
`NEXT_PUBLIC_API_URL`. Frontend implementation and testing details are in
[apps/web/README.md](apps/web/README.md).

The checked-in MapLibre basemaps use public development tile services and need
no local secret or API key. They are suitable for this repository's current
local-use stage; choose a licensed tile service before public production use.

## Optional local pipeline development

The normal runtime does not need a host Python installation. Contributors who
want IDE debugging and local pytest execution can optionally create `.venv`:

```bash
make setup
```

The following low-level commands use that optional development environment.

### Load downloaded national samples

```bash
.venv/bin/real-estate-etl sync-reference-data
.venv/bin/real-estate-etl import data/raw/ETN_SLO_2007_KPP.zip
.venv/bin/real-estate-etl import data/raw/ETN_SLO_2013_NP.zip
```

## Fetch one national dataset

Download national BUY or RENT data for one year:

```bash
.venv/bin/real-estate-etl fetch-national --type sales --year 2026
.venv/bin/real-estate-etl fetch-national --type rent --year 2026
```

Import a downloaded archive:

```bash
.venv/bin/real-estate-etl import data/raw/ETN_SLO_2026_KPP_YYYYMMDD.zip
```

## Import all national BUY and RENT history

The primary command downloads and imports every national yearly dataset:

```bash
make import-data
```

For 2007–2026 this is only 34 archives:

- 20 national BUY archives for 2007–2026
- 14 national RENT archives for 2013–2026

The command skips national dataset/year combinations already recorded in `source_files`, so it is safe to stop and restart. To retrieve current source versions again and replace existing snapshots, add `--force`:

```bash
.venv/bin/real-estate-etl refresh-all-data --force
```

To stop at a specific year, use `--end-year`.

Downloads use up to six parallel workers by default while database imports remain serial to avoid contention on shared reference and codebook tables. The importer detects the machine's logical CPU-thread count and will not allow a worker count above it. Select a different count with:

```bash
make import-data WORKERS=8
```

The startup message prints both the selected worker count and the detected logical CPU-thread count. Use fewer workers for a slower connection or to reduce load on the source API.

For troubleshooting, enable detailed per-table row counts, codebook decisions, archive paths, retries, and final timings:

```bash
make import-data-verbose WORKERS=8
```

Without `--verbose`, the importer prints concise download/completion progress plus warnings and the final summary.

The national source is preferred over municipality archives. A comparison of BUY 2007 showed that the national archive contained every unique transaction from all 212 municipality archives plus six additional transactions, while avoiding duplicated cross-municipality rows. The older `fetch`, `refresh`, and `refresh-all-sales` commands remain available only for targeted municipality diagnostics and fallback use.

The download/import pipeline is bounded to twice the worker count, so it does not queue thousands of archives in memory. Each worker keeps a reusable HTTP connection. PostgreSQL imports use `COPY`, and identical codebook CSVs are imported only once by checksum.

Transient connection resets, timeouts, HTTP 408/425/429 responses, and server errors are retried up to five times with exponential backoff. Each retry requests a fresh download token. An exhausted retry still stops safely; rerunning the command reuses completed ZIPs and skips committed database snapshots.

At the end, the command reports total elapsed time and cumulative stage timings:

```text
Finished: imported=..., already_imported=..., unavailable=..., elapsed=...,
download_worker_time=..., csv_parsing=..., database=...
```

`download_worker_time` is cumulative across parallel workers, so it can be greater than wall-clock `elapsed` time.

## Tests

Install development dependencies and run unit tests:

```bash
make test-unit
```

Run lint checks:

```bash
make lint
```

Run the isolated PostgreSQL integration test against the Docker database:

```bash
make test
```

The integration tests create uniquely named temporary schemas, test municipality and national BUY snapshots, RENT snapshots, incomplete and duplicate historical records, snapshot replacement, `COPY`, and codebook deduplication, and then remove only those temporary schemas.

## Tables

- `municipalities` and `cadastral_municipalities`: API reference data
- `transactions`: purchase transactions (`POSLI`)
- `building_parts`: building units associated with a transaction (`DELISTAVB`)
- `land_parcels`: parcels associated with a transaction (`ZEMLJISCA`)
- `rental_transactions`: rental agreements (`NP_POSLI`)
- `rental_building_parts`: rented spaces associated with rental agreements (`NP_DELISTAVB`)
- `codebook_values`: source lookup values (`sifranti`)
- `codebook_imports`: checksums of codebook versions already loaded
- `source_files`: archive checksum and import provenance
- `analytics.geographic_units`: official PostGIS boundaries and hierarchy
- `analytics.sale_facts`: one package-aware, spatially assigned row per sale
- `analytics.sale_outlier_evaluations`: versioned IQR/MAD cohort decisions

Building, parcel, and rented-space tables preserve every CSV source row, including historical unregistered records without complete cadastral identifiers or cadastral municipality codes. Reimporting a national dataset/year replaces its prior database snapshot before loading the new archive.

National BUY transaction rows do not contain `RPE_OBCINE_SIFRA`. Their locations are represented by the associated building-part and parcel rows, which contain municipality and cadastral municipality information. This also correctly supports transactions involving properties in more than one municipality.

Historical cadastral municipality codes found in source archives are retained even when they are no longer returned by the current cadastral-municipality API endpoint.

Source-system zero-date sentinels such as `00.00.0000` are stored as SQL `NULL` rather than treated as valid calendar dates.
