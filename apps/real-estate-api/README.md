# Real-estate API

Go API for Slovenian real-estate geographic and market analytics. The durable
architecture reference is [../../docs/real-estate-api.md](../../docs/real-estate-api.md),
the statistical definitions are in
[../../docs/statistical-methodology.md](../../docs/statistical-methodology.md),
and the machine-readable HTTP contract is [openapi/openapi.yaml](openapi/openapi.yaml).

## Run with Docker

From the repository root:

```bash
make api-up
curl http://localhost:8080/health/ready
curl http://localhost:8080/api/v1/meta
curl 'http://localhost:8080/api/v1/geographies?level=municipality'
```

`make api-up` starts PostGIS, applies all API/analytics migrations, builds the
Go service, and waits for both services to become healthy.

## Local Go development

Install the Go version declared in `go.mod`, then:

```bash
cd apps/real-estate-api
go test -race ./...
go vet ./...
go run ./cmd/real-estate-api
```

The code follows the official server-module convention: executable composition
is under `cmd/real-estate-api`, private packages are under `internal`, and tests remain
beside their packages as `*_test.go`. Feature handlers and repositories are
split by capability; `internal/httpapi/routes.go` contains the complete route
registry and is checked against OpenAPI. See the repository
[development guidelines](../../docs/development-guidelines.md).

The API fails fast when `DATABASE_URL` is missing or PostgreSQL is unavailable. Configuration is environment-based; see the root `.env.example` and `internal/config`.

Docker uses the dedicated `real_estate_api` PostgreSQL login. It has `SELECT`
access to source and analytical data plus narrowly scoped writes for identities,
sessions, preferences, and authenticated anomaly-review metadata. It cannot
modify imported transactions or run migrations and analytical refreshes; those
remain database-owner operations. Replace all development credentials outside
local environments.

The development container has a 512 MB ceiling. It does not load transactions or property points into memory; PostgreSQL/PostGIS executes analytical and spatial queries. The database container has a 4 GB development ceiling, while Docker Desktop should be allocated 6–8 GB overall.

## Implemented routes

- `GET /health/live`: process liveness; does not depend on PostgreSQL.
- `GET /health/ready`: readiness including PostgreSQL.
- `GET /api/v1/health`: versioned readiness endpoint.
- `GET /api/v1/meta`: supported transaction, interval, geography, and property dimensions.
- `GET /api/v1/geographies`: geographic units with optional `level` and `parentId` filters.
- `GET /api/v1/geographies/{id}`: metadata and WGS84 bounds for one unit.
- `GET /api/v1/geographies/{id}/children`: geographic hierarchy traversal.
- `GET /api/v1/geographies/{id}/boundary?detail=low|medium|full`: topology-preserving WGS84 GeoJSON boundary.
- `GET /api/v1/statistics/overview`: sale medians, averages, and explicit usable sample sizes.
- `GET /api/v1/statistics/timeseries`: monthly, quarterly, or yearly sale statistics.
- `GET /api/v1/statistics/outliers`: paginated, auditable statistical outlier evaluations.
- `GET /api/v1/transactions/extremes`: highest and lowest representative sale/rental transactions for the active market filters.
- `GET /api/v1/rentals/statistics/overview`: cleaned/raw monthly-rent and rent-per-m² metrics.
- `GET /api/v1/rentals/statistics/timeseries`: rental trends with same-period year-over-year changes.
- `GET /api/v1/rentals/statistics/outliers`: paginated, auditable rental outliers.
- `GET /api/v1/rentals/map/clusters`: bounded GeoJSON rental clusters.
- `GET /api/v1/rentals/map/geographies`: named rental summaries by administrative level.
- `GET /api/v1/rentals/map/properties`: at most 500 individual rental points.
- `GET /api/v1/sales/transactions/{id}`: sale package, component properties, geography, and quality/outlier evidence.
- `GET /api/v1/sales/transactions/{id}/context`: ranked same-building transactions, broader market benchmarks, and model context.
- `GET /api/v1/rentals/transactions/{id}`: rental package, component spaces, geography, and quality/outlier evidence.
- `GET /api/v1/rentals/transactions/{id}/context`: ranked same-building rentals, broader market benchmarks, and model context.
- `GET /api/v1/statistics/distribution`: sale percentiles and chart-ready histogram.
- `GET /api/v1/rentals/statistics/distribution`: rental percentiles and chart-ready histogram.
- `GET /api/v1/buildings/{cadastralCode}/{buildingNumber}/statistics`: combined building sale/rental history.
- `GET /api/v1/buildings/search?q=...`: address/cadastral building search for map navigation.
- `GET /api/v1/anomalies` and `/summary`: ranked multi-signal review queue.
- `PATCH /api/v1/anomalies/{kind}/{id}/review`: authenticated human review decision.
- `POST /api/v1/auth/register`, `/login`, and `/logout`: opaque bearer sessions.
- `GET /api/v1/users/me`: authenticated user profile.
- `GET|PUT /api/v1/users/me/preferences`: saved frontend and analysis preferences.
- `GET /api/v1/indices`: national repeat-sales and constant-quality hedonic indices.
- `GET /api/v1/valuations/{kind}/{id}`: model estimate, empirical range, diagnostics, and reliability warning.
- `GET /api/v1/map/clusters`: zoom-dependent GeoJSON clusters inside a required WGS84 bbox.
- `GET /api/v1/map/geographies`: named sale summaries for country, region, municipality, city district, local community, or settlement.
- `GET /api/v1/map/properties`: at most 500 individual GeoJSON sale points inside a required bbox.

Statistics accept inclusive `from`/`to` contract dates, `propertyGroup`, and
`geographyId`. Time series default to yearly periods. Metrics apply both
deterministic data-quality rules and the separate, auditable statistical
outlier stage.

Representative metrics use versioned same-building, municipality/year, or
statistical-region/year comparable cohorts with minimum sample sizes. IQR and
MAD detection includes conservative relative-dispersion floors. Responses keep
the robust median primary and distinguish `cleanedAverage*`, `rawAverage*`,
`includedSampleSize`, and `excludedOutlierCount`. Flagged source observations
remain in PostgreSQL and are available from the outlier endpoint.

Rental time series report year-over-year changes for both median monthly rent
and median monthly rent per m². Rent per m² is the preferred market comparison
because it controls for changes in rented-space size. Each point includes the
current and previous-year sample sizes and sets `weakComparisonSample=true`
when either sample has fewer than 20 representative agreements. Missing
prior-year counterparts produce null year-over-year values.

Distribution routes default to the representative per-m² metric and return
p05, p10, p25, p50, p75, p90, and p95. Histograms use the p01-p99 range so a
few extreme values do not destroy chart resolution; `belowBoundCount` and
`aboveBoundCount` retain the tails. `bins` is bounded from 5 to 100.

The anomaly queue combines named hard-rule flags, robust cohort/MAD evidence,
possible-duplicate and spatial checks, cross-fitted hedonic residual ranks, and
Isolation Forest ranks. All component scores and versions remain visible.
Machine scores create a review queue; they do not prove an error or fraud and
do not delete source rows. Authenticated reviews survive analytical refreshes.

Passwords use adaptive bcrypt hashes. Login returns a random 256-bit opaque
bearer token; PostgreSQL stores only its SHA-256 hash, and sessions expire after
30 days. The API database role cannot modify imported transactions or derived
facts. Its writes are limited to identities, sessions, preferences, and the
anomaly-review columns. Production deployments must replace development
credentials, use TLS, and apply edge rate limiting to authentication routes.

`/api/v1/indices` exposes annual matched-pair median indices and monthly
constant-quality hedonic indices. `/api/v1/valuations/{kind}/{id}` exposes an
empirical 90% range for an observed eligible transaction, model diagnostics,
and a reliability grade. These are statistical estimates, not official
valuations. Current land-model error is high and is deliberately reported as
`very_low` reliability.

Map routes require `bbox=west,south,east,north` and use the PostGIS point index.
Clusters change grid resolution with `zoom` and are capped at 5,000 cells.
Individual points default to 250 and cannot exceed 500. Date, property, and
geography filters are shared with the statistics routes.

Populate or refresh the geography hierarchy with `real-estate-etl
sync-geographies`.

After importing source sales and synchronizing boundaries, rebuild the
database-backed facts with:

```bash
docker compose exec -T db psql -U real_estate -d real_estate \
  -c "SELECT analytics.refresh_sale_facts();" \
  -c "SELECT analytics.refresh_land_transaction_facts();" \
  -c "SELECT analytics.refresh_sale_outliers();" \
  -c "SELECT analytics.refresh_rental_facts();" \
  -c "SELECT analytics.refresh_rental_outliers();" \
  -c "SELECT analytics.refresh_anomaly_signals();" \
  -c "SELECT analytics.refresh_near_duplicate_candidates();" \
  -c "SELECT analytics.refresh_missingness_profiles();" \
  -c "SELECT analytics.refresh_repeat_sales_indices();"
```

The calculation runs inside PostgreSQL/PostGIS; the API never caches the full
transaction or geography dataset in application memory.

The root shortcut is:

```bash
make analytics-refresh
make models-refresh WORKERS=6
```

The first command refreshes sale/rental facts, strict land readiness, robust
outliers, missingness, duplicate candidates, and the accurately named
matched-pair median index. The second evaluates property-group hedonic models
on the latest complete year and municipality holdouts, calibrates intervals
on the preceding year, trains deterministic Isolation Forest models, and
appends a hedonic-index vintage.
Rental amounts are treated as monthly, and only homogeneous market-rent
packages passing explicit quality and robust comparable-cohort rules are
representative. Raw source observations are never removed.

Sale endpoints accept `landScope=strict|broad|all`. The default `strict`
population includes land-only contracts containing exclusively ETN land codes
1 and 2. `broad` permits only codes 1–3; `all` is explicit audit access. The
scope has no effect for other property groups.

## Design principles

- one shared PostgreSQL/PostGIS database, with API-derived objects isolated in the `analytics` schema;
- source tables are never mutated by the API;
- explicit OpenAPI contract and versioned routes;
- bounded connection pool and request/server timeouts;
- graceful shutdown and fail-fast startup;
- structured JSON logs, request IDs, panic recovery, CORS allowlist, and security headers;
- small handlers backed by repository interfaces for isolated testing;
- no ORM: spatial and analytical SQL remains visible and reviewable.
