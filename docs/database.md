# Database architecture

## Land browsing categories

Migration `037_land_browsing_categories.sql` adds an immutable classification
function `analytics.land_market_category(integer[],integer)` and the read-only
`analytics.land_market_categories` view over published `land_transaction_facts`.
Five sale-fact views expose agricultural, forest, unverified building land, mixed
and other/unknown populations. Existing strict/broad/all views are unchanged.
All category views retain non-land rows for compatibility; `propertyGroup=land`
selects the land population. A land sale lacking any land-fact classification
belongs to the other/unknown view. API access is SELECT-only.

No backfill, analytical refresh or model retraining is required: categories are
derived from the existing distinct ETN codes and unknown-parcel count at query
time. Published analytical releases therefore remain the source of truth.

Production uses CloudNativePG on K3s, not the local Compose database lifecycle.
See [deployment and privileged migration Jobs](k3s-deployment.md) and
[backup/PITR](backup-and-recovery.md). Authoritative schema SQL remains here;
production additionally records per-file checksums and applies the versioned
production privilege policy. The API never performs production migrations.

## Purpose and ownership

PostgreSQL 16 with PostGIS is the system of record. It stores structured GURS
source rows, current GURS RPE geography, deterministic analytical facts,
offline-model outputs, application identity, preferences, and anomaly reviews.
Neither the API nor the frontend maintains a second copy of market data.

Schema changes are ordered SQL files in `database/migrations`. The database
container applies them on a new volume; `make db-migrate` safely reapplies the
idempotent incremental migrations to an existing volume.

## Logical schemas

### `public`: source fidelity and application state

- `source_files` is the import/provenance ledger (scope, product, year,
  checksum, and timestamps).
- `transactions`, `building_parts`, and `land_parcels` preserve BUY/KPP
  agreement and component rows.
- `rental_transactions` and `rental_building_parts` preserve RENT/NP rows.
- `codebook_values` and `codebook_imports` preserve GURS categorical meaning.
- `municipalities` and `cadastral_municipalities` are source references.
- `app_users`, `auth_sessions`, and `user_preferences` are application-owned.

Public ETN party names and identifiers are not present in the source archives
or schema. Unstructured GURS note fields are intentionally excluded from import
and removed by migration 027 because they are unnecessary for analytics and
could theoretically contain identifying free text. This is the sole deliberate
exception to structured source-field preservation. See
[`data-licensing-privacy.md`](data-licensing-privacy.md).

Source child rows use internal identities plus `(source_file_id,
source_row_number)` uniqueness because historical cadastral identifiers may be
missing. Negative GURS transaction identifiers are valid. Empty values remain
null; analytical code never invents missing prices, rents, areas, or locations.

### `analytics`: query-ready and derived data

- `geographic_units` stores the country-to-settlement hierarchy and PostGIS
  polygons in EPSG:3794 and EPSG:4326.
- `sale_facts` and `rental_facts` contain one row per agreement, preventing a
  house, garage, and parcel package from being counted as three transactions.
- `land_transaction_facts` and the strict/broad/all views encode explicit land
  populations.
- outlier evaluations, anomaly signals, near-duplicate candidates, and
  missingness profiles retain explainable quality evidence.
- `sale_package_signature` and `rental_package_signature` derive deterministic
  multisets of ETN component-type codes for exact comparable-package matching;
  they do not replace or modify the raw component rows.
- model runs, valuation predictions, index calculation runs, market indices,
  and repeat-sale pairs retain versioned analytical outputs.
- `refresh_runs` records deterministic refresh status and diagnostics.

### Atomic analytical publication

`analytical_releases` records source cutoffs, published analytical cutoffs,
model versions, processing stages, and outcomes. `analytical_publication` is a
singleton pointer to the active release and any pending replacement. Raw GURS
imports commit first; facts, outlier classifications, shared analytics,
indices, model predictions, and the release pointer then change inside one
PostgreSQL transaction. API readers therefore retain the previous committed
generation until all required work succeeds.

Predictions are immutable per model run. `model_publications` selects the
active sale and rental generations and `current_valuation_predictions` is the
API-facing view. Pending or failed training never overwrites active model
predictions.

Repeat-sale pairs are fully derived. Both links to `sale_facts` cascade on
deletion so replacing a yearly GURS archive safely invalidates obsolete pairs;
the analytics refresh recreates them after the fact tables are current.

## Spatial model

Source centroids are D96/TM coordinates (EPSG:3794). Generated `location`
columns create PostGIS points only when both coordinates exist and have GiST
indexes. Fact refreshes aggregate component points to one representative
transaction centroid. One settlement spatial lookup supplies settlement,
municipality, and statistical-region ancestry; city districts and local
communities remain independent spatial lookups because they are sibling RPE
levels whose polygons do not always nest perfectly. WGS84 transformations are
performed for browser maps.

These are source centroids, not guaranteed entrances. Current polygons do not
reconstruct historical administrative boundaries. Missing locations remain
observable in coverage metrics but cannot appear on a point map.

## Data lifecycle

```text
GURS ZIP/CSV
  -> source_files + public source tables
  -> current RPE geographic_units
  -> transaction-grain facts and quality evaluations
  -> anomaly/missingness/duplicate tables and matched-pair indices
  -> offline models, predictions, Isolation Forest scores, hedonic indices
  -> read-only API queries
```

The worker takes a PostgreSQL advisory lock, imports each archive
transactionally, then refreshes derived objects in dependency order. A failed
step does not expose a partially imported archive. The normal scheduled run is
every 14 days and also rechecks the current year for revisions. A new database
trains its initial valuation models once. After that bootstrap, training is
available only through the explicit `make models-refresh` command.

`make reset-and-repopulate` truncates market-source and derived analytical
tables with explicit confirmation, then rebuilds them. Users and saved
preferences are preserved. Valuation runs and predictions are removed and are
not automatically recreated because the durable installation bootstrap marker
is preserved. Docker volumes survive normal container rebuilds;
deleting the database volume also deletes persisted model output, which can be
recovered only from a database backup or by explicit model retraining.

The local PostgreSQL container is tuned within its 4GB limit for full-table
analytical rebuilds. Independent sale and rental chains run concurrently on two
connections; shared anomaly, duplicate, missingness, and repeat-index stages run
only after both chains succeed. Each chain is transactional, but the complete
cross-chain refresh is not one transaction, so readers can briefly observe mixed
generations during a scheduled rebuild. Fact-table refresh functions truncate
and repopulate their tables in a transaction; API reads of those tables can
wait on PostgreSQL relation locks until the corresponding sale or rental branch
commits. The worker prints a periodic heartbeat for long SQL stages, followed
by the exact row count and duration. Individual `analytics.refresh_runs` stages
also use wall-clock start timestamps.

## Access and integrity

Migrations 038/039 add `analytics.property_history_events` and
`analytics.property_resale_pairs` views plus three source identity/reference
indexes. They reuse published sale facts and current source components; no
ETN records, valuations, outlier flags or index populations are modified.
History totals come from reported `transactions.contract_price`, not derived
fact prices. See [property-history definitions](property-history.md) for exact
identity matching, share/package safeguards and publication rules.

Foreign keys connect agreements, source files, and components. Check
constraints encode enumerations and valid review states. B-tree indexes serve
dates, property groups, geographic identifiers, and cadastral building keys;
GiST serves spatial filtering; GIN serves anomaly flags.

The pipeline connects as the owner role. `real_estate_api` is least-privilege:
it reads source/reference/analytics data and can write only identity,
preferences, sessions, and reviewed anomaly decisions. API requests cannot
truncate, import, migrate, or refresh analytical tables.

## Operations

```bash
make db-up                 # start PostGIS and apply migrations
make db-migrate            # reapply incremental migrations
make analytics-refresh     # deterministic database-derived objects
make models-refresh        # explicit retraining after first-install bootstrap
make reset-and-repopulate  # destructive market-data rebuild, preserves users
```

Default local connection: `postgresql://real_estate:real_estate@localhost:5432/real_estate`.
The complete source-column reference remains in
`database/documentation/data-dictionary.md`; statistical population and
quality definitions are in `docs/statistical-methodology.md`.
