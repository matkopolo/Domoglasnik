# Slovenian real-estate analytics monorepo

This project downloads national yearly ZIP archives from GURS JGP, preserves the raw files, and imports their CSV contents into PostgreSQL. It loads both purchase transactions (BUY/KPP, 2007 onward) and rental transactions (RENT/NP, 2013 onward). Imports are idempotent: reimporting a dataset/year replaces that database snapshot instead of duplicating it.

## Project structure

- `apps/data-pipeline/`: Python downloader, transformer, PostgreSQL loader, and tests
- `apps/api/`: future Go backend application
- `apps/web/`: future frontend application
- `database/migrations/`: shared PostgreSQL schema and incremental migrations
- `database/documentation/`: shared data-model documentation
- `infrastructure/docker/`: future service-specific Docker assets
- `data/raw/`: local downloaded archives, excluded from Git
- `Makefile`: root commands shared by all developers

Inside the data pipeline:

- `api.py`: `JGPClient`, pooled HTTP connections, atomic ZIP downloads, and retry policy
- `settings.py`: immutable API/product metadata and machine defaults
- `models.py`: typed results passed between download, parsing, and loading stages
- `application.py`: declarative CSV-to-column mappings, PostgreSQL `COPY`, bounded orchestration, and command handlers
- `cli.py`: minimal executable entry point
- `pyproject.toml`: installable package metadata, console command, dependencies, and Ruff rules

The PostgreSQL tables remain in `database/migrations`, not Python. CSV mappings are intentionally kept as one declarative registry rather than one file per table: they are source-to-destination translation rules, and keeping BUY and RENT mappings together makes schema drift comparisons and validation easier. If the number of source products grows substantially, that registry can be moved intact into `mappings.py` without changing the importer.

## Library choices

- **[HTTPX](https://www.python-httpx.org/)** provides connection pooling, streaming downloads, and explicit timeout/status handling.
- **[Tenacity](https://tenacity.readthedocs.io/en/stable/)** provides bounded exponential retry behavior for transient transport and server failures.
- **psycopg 3** streams rows through PostgreSQL `COPY`; SQLAlchemy and pandas are intentionally not used because they would add memory and abstraction overhead to the bulk-loading path.
- **pytest** covers conversion, retry, resume, historical edge cases, and isolated PostgreSQL integration.
- **[Ruff](https://docs.astral.sh/ruff/)** enforces imports, correctness rules, modernization, and style locally and in GitHub Actions.

The package installs a `real-estate-etl` executable, so developers do not need to manage `PYTHONPATH` manually.

## Start PostgreSQL

```bash
make setup
make db-up
```

The default connection is `postgresql://real_estate:real_estate@localhost:5432/real_estate`. Copy `.env.example` to `.env` or export `DATABASE_URL` if you change it.

## Windows setup

Install Docker Desktop for Windows and Python 3.9 or newer. Then open PowerShell in the project folder and run:

```powershell
docker compose up -d db
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -e "apps/data-pipeline[dev]"
.\.venv\Scripts\real-estate-etl.exe refresh-all-data
```

If PowerShell blocks virtual-environment activation, enable it for only the current PowerShell process:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\.venv\Scripts\Activate.ps1
```

Windows does not provide the macOS `caffeinate` command. Disable automatic sleep in **Settings → System → Power & battery → Screen and sleep** while a full import is running.

Do not copy `.venv` or a local PostgreSQL volume between computers. Share the source project; each user should create their own virtual environment and Docker database with the commands above.

## Load downloaded national samples

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

PowerShell equivalent:

```powershell
$env:TEST_DATABASE_URL = "postgresql://real_estate:real_estate@localhost:5432/real_estate"
Set-Location apps/data-pipeline
..\..\.venv\Scripts\pytest.exe -q -m integration
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

Building, parcel, and rented-space tables preserve every CSV source row, including historical unregistered records without complete cadastral identifiers or cadastral municipality codes. Reimporting a national dataset/year replaces its prior database snapshot before loading the new archive.

National BUY transaction rows do not contain `RPE_OBCINE_SIFRA`. Their locations are represented by the associated building-part and parcel rows, which contain municipality and cadastral municipality information. This also correctly supports transactions involving properties in more than one municipality.

Historical cadastral municipality codes found in source archives are retained even when they are no longer returned by the current cadastral-municipality API endpoint.

Source-system zero-date sentinels such as `00.00.0000` are stored as SQL `NULL` rather than treated as valid calendar dates.
