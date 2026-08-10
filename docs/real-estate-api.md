# Real-estate API service

## Responsibility

`apps/real-estate-api` is a Go HTTP service over PostgreSQL/PostGIS. It exposes
bounded, versioned JSON/GeoJSON endpoints for market analytics, maps,
transactions, buildings, anomalies, indices, valuations, geography, identity,
and preferences. It does not download GURS data or fit statistical models.

Public routes remain under `/api/v1`; the executable and Docker Compose
service are named `real-estate-api`.

## Architecture

```text
cmd/real-estate-api
  -> config + database pool
  -> httpapi (routing, validation, middleware, JSON)
  -> analytics / geography / identity repositories
  -> PostgreSQL public + analytics schemas
```

Packages under `internal` enforce the application boundary. Handlers parse and
validate input, delegate to repositories, and serialize stable DTOs. SQL owns
filtering and aggregation. The service uses a bounded pgx pool, request IDs,
structured logs, panic recovery, CORS allowlisting, security headers, server
timeouts, graceful shutdown, and readiness checks.

## Endpoint groups

- Health and capabilities: `/health/live`, `/health/ready`, `/api/v1/health`,
  `/api/v1/meta`.
- Geography: list, detail, children, and topology-preserving boundaries.
- Sale/rental statistics: overview, time series, distributions, outliers, and
  transaction extremes.
- Maps: bounded clusters, named geographic summaries, and capped property
  points for sales and rentals.
- Transactions/buildings: agreement composition, contextual comparables,
  cadastral building history, and address search.
- Quality: anomaly list/summary and authenticated review decisions.
- Models: market indices and versioned transaction valuation estimates with
  uncertainty and diagnostics.
- Economy: `/api/v1/economy/inflation` proxies the official SURS SiStat
  monthly inflation table `H281S`, supports monthly, quarterly, and yearly
  output, and caches the external response for six hours. Inflation is
  national context and is never filtered by property or geography.
- Identity: registration, login/logout, current user, and saved preferences.

The authoritative operation and schema definitions are in
`apps/real-estate-api/openapi/openapi.yaml`.

## Statistical contract

The API distinguishes raw/quality-eligible/representative samples and returns
sample sizes with publication status. Medians and percentiles are primary;
averages are supplementary. Outlier removal is limited to the documented
representative population. A high anomaly review score alone never deletes a
source row. Land defaults to the strict ETN buildable-code population; broader
land scopes require an explicit query parameter.

Common filters include contract dates, property group, geography, land scope,
and inclusive minimum/maximum area. Map requests additionally require a valid
WGS84 bounding box and enforce result limits.

Transaction context is returned by
`GET /api/v1/sales/transactions/{id}/context` and
`GET /api/v1/rentals/transactions/{id}/context`. It uses one hierarchy for
every sale and rental. It first
returns quality-eligible transactions from the same building: same year and
area within ±15%, adjacent year within ±15%, then other same-year units. A
nearby benchmark is selected only from units within ±15% area and ±10
construction years; when that is too thin it widens to ±25% area and ±15
construction years. Settlement and other small-area context is likewise
physically matched. Municipality, statistical-region, and national medians are
deliberately broader market context. Each aggregate response exposes the
matching profile and tolerances that produced it. The model estimate remains a
separate final reference rather than being mixed into observed comparables.
The same response adds an adaptive current-market estimate based on weighted
latest-market transactions. This is a backend comparable calculation, not the
trained model returned in the separate `valuation` field. It first uses the
current calendar year, expanding through 1, 2.5, 5, 10, and 20 km and from
strict to wider area/building-age tolerances. Only if no current-year cohort
reaches the 50-comparable target does it combine the previous and current
calendar years. It is not a ±1-year window around the subject transaction.
The result returns weighted p25/median/p75 values, sample size, actual radius,
physical tolerances, municipality adjustment, confidence and explicit
fallback warnings. The complete search order, weights, formulas, year example,
and limitations are documented under “Current-market estimate” in
`docs/statistical-methodology.md`.

See `docs/statistical-methodology.md` for equations, cohorts, model validation,
uncertainty, and known limitations.

## Security and persistence

Public analytical routes are read-only. Passwords are bcrypt hashes; sessions
use opaque bearer tokens whose hashes are stored with expiry. The database
role is denied source/analytics mutation and has narrow writes for users,
sessions, preferences, and review state. Authentication is not a claim that
model outputs are appraisals or investment advice.

## Development

```bash
make api-up
make api-fmt
make api-vet
make api-test
make api-build
```

The convenience targets retain the familiar `api-*` spelling, but operate on
the `real-estate-api` Compose service. The service is available at
<http://localhost:8080>; start with `/health/ready`. Go tests are colocated with
their packages, while database-backed integration tests use
`TEST_DATABASE_URL` and run with the race detector in CI.
