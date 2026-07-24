# Code Quality Audit and Codex Remediation Plan

## Scope and limitations

- Repository: `matkopolo/NepremicninskiAgent`
- Branch: `main`
- Reviewed source state: commit `70b5ec82e964e2cf5061aa6855fb2a6b16bfe610`
- Review type: static repository review
- Components reviewed separately:
  1. Python data pipeline / data stream
  2. Go API
  3. Next.js / TypeScript web application
  4. Cross-layer consistency

No successful GitHub Actions run was available through the connector for the reviewed commit. Codex must run the complete local/CI verification suite before and after every change. The ratings below assess the inspected source and tests; they do not certify runtime correctness or production readiness.

## Rating scale

- `9-10`: production-quality structure with only minor refinements
- `8-8.9`: strong implementation with contained technical debt
- `7-7.9`: solid foundation with meaningful maintainability or correctness work
- `6-6.9`: usable, but important refactoring and test gaps exist
- below `6`: material structural or correctness risk

## Executive scorecard

| Component | Consistency | Readability | Testability | Robustness / correctness | Overall |
|---|---:|---:|---:|---:|---:|
| Python data pipeline | 7.4 | 7.1 | 6.7 | 7.4 | **7.2 / 10** |
| Go API | 8.1 | 7.8 | 8.0 | 7.1 | **7.7 / 10** |
| Next.js web | 7.1 | 6.4 | 6.4 | 6.4 | **6.6 / 10** |
| Cross-layer consistency | 7.2 | 7.1 | 7.0 | 6.9 | **7.1 / 10** |
| Whole repository | — | — | — | — | **7.2 / 10** |

The repository is an ambitious and generally well-structured foundation. It should be improved incrementally rather than rewritten. The API is currently the strongest layer. The largest readability problem is the map component. The largest methodological problem is the model-refresh module. The clearest functional defect is browser CORS configuration for authenticated write routes.

---

# 1. Python data pipeline / data stream

## Rating

**Overall: 7.2 / 10**

### Strong points

- The package is divided into recognisable responsibilities: resilient HTTP transport, importing, orchestration, geography ingestion, modeling, CLI, configuration, and shared models.
- Downloads are atomic and retry only transient failures.
- Download concurrency is bounded while database imports remain sequential on the owning connection.
- Imports record source-file hashes and source-row provenance.
- PostgreSQL `COPY` is used for bulk loading.
- Geography refreshes are transactional and record refresh-run success/failure.
- Integration tests use isolated schemas and verify preservation of incomplete and duplicate source rows.
- Ruff, pytest, and database-backed integration tests are wired into CI.

## Findings and Codex actions

### DATA-P0 — Stream archive members instead of loading full CSVs into memory

**Evidence**

`importer.py` currently calls `archive.read(member)`, decodes the complete member, and then creates `list(csv.DictReader(...))`. This conflicts with the repository guideline that the complete source dataset must not be loaded into memory.

**Required change**

- Open ZIP members with `ZipFile.open`.
- Wrap the binary stream in `io.TextIOWrapper(encoding="utf-8-sig", newline="")`.
- Feed the streaming `DictReader` directly into a bounded `COPY` path.
- Preserve the source row number.
- Calculate row counts without materializing every row.
- Handle codebook imports in bounded batches if `executemany` would otherwise require a complete list.
- Add a regression test using a generated large CSV and an assertion or instrumentation proving bounded iteration.
- Do not weaken unknown-column validation.

### DATA-P0 — Make source conversion strict and diagnostically useful

**Evidence**

- Boolean conversion currently treats only `1`, `true`, and `TRUE` as true; every other non-empty value becomes false.
- Numeric values are returned as strings after decimal-comma replacement.
- Conversion errors do not include archive, member, row, source column, and raw value.

**Required change**

- Introduce a typed `TableSpec` / `FieldSpec` registry.
- Accept only documented boolean tokens. Unknown non-empty tokens must raise a source-data validation error or create an explicit import-quality signal; never silently coerce them to false.
- Parse numeric values with `Decimal` where exact monetary/area semantics matter, or document why PostgreSQL textual numeric input is intentionally retained.
- Raise a structured exception containing archive name, CSV member, source row, source field, destination field, and raw value.
- Add table-driven tests for valid values, blank values, zero-date sentinels, decimal commas, malformed numbers, and unsupported booleans.

### DATA-P1 — Separate mapping configuration from import orchestration

**Evidence**

`importer.py` contains the full BUY and RENT field registries, conversion policy, ZIP-name interpretation, codebook handling, replacement semantics, CSV parsing, cadastral reference updates, and database COPY logic.

**Required change**

Split only along real reasons to change:

- `mappings.py`: immutable typed BUY/RENT table specifications
- `conversion.py`: pure source-value conversion and diagnostic errors
- `archive.py` or `archive_metadata.py`: archive-name/type/scope/year parsing
- `importer.py`: transaction orchestration and database writes

Do not create many one-function files. Preserve the public compatibility façade until callers and tests have migrated.

### DATA-P1 — Decompose `modeling.py` into independently testable responsibilities

**Evidence**

The current module performs dataset extraction, feature construction, shuffled cross-validation, Ridge fitting, Isolation Forest fitting, interval construction, anomaly score persistence, valuation persistence, and hedonic-index generation in one approximately 460-line module.

**Required target structure**

- `model_dataset.py`: SQL extraction and typed model rows
- `features.py`: feature definitions and preprocessing
- `validation.py`: temporal, spatial, and cross-fitting splitters and metrics
- `hedonic.py`: transparent baseline and Ridge model
- `anomaly_models.py`: Isolation Forest and residual ranking
- `prediction_intervals.py`: held-out/conformal calibration
- `indices.py`: index construction
- `model_persistence.py`: database writes and model-run metadata
- `modeling.py`: a short orchestration façade, if compatibility requires it

The methodology guide remains authoritative for temporal/spatial validation and honest interval coverage.

### DATA-P1 — Expand model tests substantially

Current direct unit coverage is too small relative to the amount of statistical logic.

Add deterministic tests for:

- feature order and numeric/categorical partitions;
- missing-value behavior and missingness indicators;
- property-group separation;
- temporal split leakage;
- spatial-block leakage;
- cross-fitted predictions never scoring their own training row;
- percentile ranking with ties;
- interval calibration and held-out coverage calculation;
- small/empty property groups;
- deterministic random seeds;
- index base period, period sample size, and missing-period handling;
- persistence rollback on failure.

### DATA-P2 — Improve orchestration contracts and logging

- `_run_refresh` should accept a `Sequence`, or materialize an arbitrary iterable once. The current `len` fallback can submit zero jobs for a non-sized iterable.
- Replace direct `print` calls with structured logging while preserving a human-readable CLI mode.
- Add explicit download/import outcome records so automation does not need to parse console output.
- Add a documented removal date or release milestone for the `application.py` compatibility façade.

---

# 2. Go API

## Rating

**Overall: 7.7 / 10**

### Strong points

- Idiomatic executable wiring under `cmd/api`.
- Environment configuration is validated and database startup fails fast.
- Graceful shutdown and server timeouts are configured.
- Handlers use request context.
- Errors are wrapped at repository boundaries and mapped to stable HTTP errors.
- Interfaces are generally defined at consumer boundaries.
- Request bodies are size-bounded and decoded strictly.
- Passwords are bcrypt-hashed; random session tokens are stored only as SHA-256 hashes.
- Central route registration is checked against the OpenAPI path/method set.
- Unit, handler, integration, race, and OpenAPI checks provide broad coverage.

## Findings and Codex actions

### API-P0 — Fix CORS for authenticated write operations

**Functional defect**

The middleware currently advertises only:

- methods: `GET, OPTIONS`
- headers: `Accept, Content-Type, X-Request-ID`

The API exposes browser-facing `POST`, `PUT`, and `PATCH` routes, and authentication requires the `Authorization` header. A cross-origin browser can therefore reject login, registration, logout, preference updates, and anomaly review during preflight.

**Required change**

- For an allowed origin, advertise the actually supported methods: at least `GET, POST, PUT, PATCH, OPTIONS`; include `DELETE` only when a route requires it.
- Allow `Authorization`, `Accept`, `Content-Type`, and `X-Request-ID`.
- Set `Vary: Origin`, and also vary on preflight request method/headers where appropriate.
- Do not use wildcard origins with credentials or bearer-auth assumptions.
- Reject or safely ignore disallowed origins.
- Add table-driven preflight tests for:
  - login/register;
  - authenticated preferences;
  - authenticated anomaly review;
  - disallowed origin;
  - unsupported requested header/method.

### API-P1 — Split the growing all-purpose analytics repository

One `StatisticsRepository` currently satisfies statistics, rentals, transaction details, distributions, buildings, anomalies, models, and map interfaces. HTTP interfaces are small, but the concrete repository has become a broad change surface.

Refactor incrementally into capability-oriented repositories or focused query objects sharing the same pool:

- sale statistics;
- rental statistics;
- distributions;
- map queries;
- transaction/building detail;
- anomaly review;
- valuation/index queries.

Do not create a generic `utils` or abstract SQL framework. Keep analytical SQL visible and reviewable.

### API-P1 — Remove repeated SQL and tautological geography clauses

- Overview and time-series queries repeat the same aggregate columns.
- Geography filtering returns expressions such as `($4::bigint IS NULL OR true)` and `($4::bigint IS NOT NULL OR true)`, which are always true and obscure intent.
- Dynamic table/column fragments are currently allow-listed, but their safety depends on maintaining that discipline.

Required change:

- Introduce a small, tested query-fragment builder or shared aggregate column constant local to the analytics package.
- Return a plain `TRUE` clause when no geography predicate is needed.
- Centralize allowed geography fact-column mapping.
- Add tests for country, region, municipality, district, community, settlement, unknown level, and nil geography.
- Keep all user values as parameters; only fixed allow-listed identifiers may be interpolated.

### API-P1 — Strengthen contract testing beyond path parity

The OpenAPI test confirms route method/path parity and operation-ID uniqueness, but does not prove that runtime response bodies match documented schemas.

Add:

- OpenAPI structural validation using a maintained validator;
- representative handler-response validation against schemas;
- generated or shared contract fixtures;
- tests for error envelopes, pagination, nullable fields, and GeoJSON content types;
- no hard-coded expected path count that must be manually changed for every legitimate new endpoint.

### API-P2 — Improve middleware correctness and observability

- `statusRecorder.WriteHeader` should ignore repeated calls after the first status.
- Preserve optional `http.ResponseWriter` capabilities required by the chosen Go version and server behavior, or use `http.NewResponseController` where appropriate.
- Include request ID in structured error responses.
- Consider logging route pattern rather than only raw path to reduce high-cardinality logs.
- Add request-duration and database-query metrics when observability is introduced.
- Add per-route rate limiting or edge requirements for authentication endpoints.

### API-P2 — Keep domain values synchronized

Property groups, geography levels, intervals, and transaction kinds are repeated across SQL checks, Go validation, `/meta`, OpenAPI, and TypeScript.

Preferred direction:

- define authoritative domain constants in Go and SQL migrations;
- expose supported values through `/meta`;
- generate frontend contract types from OpenAPI;
- add consistency tests rather than creating a runtime dependency from the database to every validator.

### API-P2 — Replace the duplicated migration list

`db-up` and `db-migrate` manually repeat every migration filename. A new migration can be committed but omitted from one target.

Use either:

- a simple ordered migration loop backed by a schema-migrations table and checksum; or
- a small established migration tool.

CI must verify that a clean database reaches the latest schema and that rerunning migrations is safe according to the chosen policy.

---

# 3. Next.js / TypeScript web application

## Rating

**Overall: 6.6 / 10**

### Strong points

- Strict TypeScript is enabled.
- The top-level dashboard is composed from focused presentation components.
- Controls use native accessible elements, labels, focus states, and Slovenian UI text.
- API access is centralized.
- Map API behavior and dashboard switching have useful tests.
- The UI visibly distinguishes live and demonstration sources, although the fallback behavior needs stronger safeguards.
- The visual design is consistent and uses reusable CSS variables.

## Findings and Codex actions

### WEB-P0 — Do not silently replace failed live data with demo data

`loadDashboard` catches every non-abort request error and returns demo data. In a production analytics product this can present plausible but false values after an API failure.

**Required behavior**

- Demo fallback must be controlled by an explicit development/demo setting such as `NEXT_PUBLIC_ENABLE_DEMO_FALLBACK=true`.
- In normal operation, return a typed error and show an obvious error/offline state.
- Preserve the last successful live result when appropriate, clearly marked stale.
- Show data source, data vintage, and last successful refresh time.
- Never label demo values as verified market data.
- Add tests for live success, abort, API error, stale live data, enabled demo mode, and disabled demo mode.

### WEB-P0 — Handle aborts and stale async responses explicitly

- `MarketDashboard` has no `.catch` before `.finally`; an aborted request is rethrown by `loadDashboard`.
- Building selection requests create abort controllers that are not retained and can resolve after a newer selection.
- Similar map-popup requests need consistent cancellation ownership.

Required change:

- introduce a typed API error including `status`, `code`, and abort detection;
- ignore expected `AbortError` without an unhandled rejection;
- abort prior selection/search/detail requests;
- reject stale responses using request generation IDs or owned controllers;
- test rapid filter changes, rapid building selection, popup close, style change, and component unmount.

### WEB-P1 — Refactor `slovenia-map.tsx`

This is the largest readability hotspot. The component is roughly 700 lines and combines:

- MapLibre lifecycle;
- data fetching;
- boundary synchronization;
- marker creation;
- duplicate marker event/popup logic;
- building search;
- transaction loading;
- imperative popup DOM construction;
- style controls;
- semantic zoom;
- view rendering.

The marker creation and popup behavior are implemented more than once.

**Suggested decomposition**

- `useMapLibreMap.ts`: map creation, teardown, style lifecycle
- `useSemanticMapData.ts`: bounds/zoom-driven fetch, cancellation, status
- `map-markers.ts`: one marker construction/update implementation
- `map-popups.ts`: safe popup DOM factories
- `building-search-panel.tsx`: search/result card UI
- `slovenia-map.tsx`: composition and state coordination only

Keep imperative MapLibre integration where it belongs; do not force it into JSX if that reduces clarity. The goal is one marker rendering path and independently testable pure helpers.

### WEB-P1 — Generate API types from OpenAPI

The frontend manually duplicates many Go/OpenAPI response shapes and uses broad `Record<string, ...>` GeoJSON properties.

Required change:

- generate TypeScript contract types from `apps/api/openapi/openapi.yaml`;
- keep UI-specific view models separate from transport models;
- add a reproducible generation command and CI drift check;
- optionally validate untrusted runtime payloads at the API boundary for critical endpoints;
- avoid `return response.json() as Promise<T>` as the only contract guarantee.

### WEB-P1 — Remove hard-coded data dates and ranges

- The dashboard displays a hard-coded update date.
- The default year range ends at 2025.
- Source freshness should come from the API/data refresh metadata.

Add an API data-vintage/meta response containing:

- latest successful import/analytics/model refresh;
- available minimum and maximum contract years by mode;
- methodology/rule version;
- source state.

Use that response to populate defaults and the “updated” label.

### WEB-P2 — Increase frontend test depth

Add tests for:

- map data hook cancellation and stale-response prevention;
- one marker renderer for geography/building/property modes;
- popup content formatting and missing values;
- building-search debounce and cancellation;
- source/error/stale/demo banners;
- strict building-plot defaults for land codes 1–2;
- keyboard navigation and basic accessibility;
- responsive behavior of the key analysis flow.

MapLibre itself may be mocked, but the adapter and lifecycle logic must be tested rather than mocking the entire `SloveniaMap` component in every dashboard test.

### WEB-P2 — Consolidate package management

The repository has both root and `apps/web` package manifests/lockfiles with duplicated web dependencies.

Choose and document one model:

- npm workspace with root commands delegating to `apps/web`; or
- a deployment-only root wrapper generated from/locked to the web package.

Prevent version and override drift with CI.

### WEB-P2 — Type-check test files

Production TypeScript is strict, but test files are excluded from the main `tsconfig`.

Add a test TypeScript configuration and a CI `typecheck` command covering application and test code.

---

# 4. Cross-layer consistency

## Rating

**Overall: 7.1 / 10**

## Strong points

- The monorepo has clear top-level ownership.
- Naming is mostly consistent across SQL, Go JSON, and TypeScript.
- Routes have a single Go registration point and OpenAPI parity test.
- CI covers Python, database migrations, Go formatting/vetting/build/race tests, web lint/tests/build.
- Documentation explains conventions and analytical safeguards.

## Consistency gaps to fix

### CONSISTENCY-P0 — Preserve the verified-buildable default everywhere

The methodology instruction is mandatory:

- retain every land transaction;
- default all building-plot statistics and views to strict verified-buildable ETN codes 1 and 2;
- store and expose code 3 separately;
- do not let code 3, mixed, unknown, or non-buildable land affect the default median, averages, distributions, maps, trends, indices, or model populations.

Codex must trace this rule through:

1. analytical facts;
2. eligibility/outlier logic;
3. API overview/time-series/distribution/map endpoints;
4. OpenAPI descriptions and enums;
5. frontend defaults and labels;
6. model extraction;
7. tests at every layer.

### CONSISTENCY-P1 — Eliminate manual transport-model duplication

Use OpenAPI generation and consistency tests so a field rename or nullability change cannot silently diverge between Go and TypeScript.

### CONSISTENCY-P1 — Align written guidelines with executable checks

Examples:

- guidelines forbid loading complete source datasets into memory, while the importer currently materializes each complete CSV member;
- guidelines require `npm audit`, while CI does not run it;
- code-quality requirements should be enforced by commands, not documentation alone.

Codex must either implement the check or revise an absolute guideline to describe the real policy.

### CONSISTENCY-P1 — Define shared quality gates

Add or document:

- Python coverage target focused on conversion/modeling;
- Go coverage reporting plus `staticcheck`; consider `govulncheck`;
- frontend `typecheck`, coverage, and dependency audit policy;
- migration completeness/checksum verification;
- OpenAPI-generated-client drift check;
- SQL performance regression checks for key map/statistics queries.

Do not pursue one arbitrary repository-wide percentage. Set meaningful per-layer targets and require high coverage for pure correctness-critical logic.

---

# 5. Prioritized Codex execution order

## Phase A — Correctness before refactoring

1. Fix and test CORS for authenticated browser routes.
2. Replace silent production demo fallback with explicit error/stale/demo states.
3. Add abort/stale-response handling in dashboard and map requests.
4. Enforce strict verified-buildable codes 1–2 across all default land statistics and views.
5. Correct model validation and prediction-interval methodology as required by the methodology guide.

## Phase B — Highest-value decomposition

6. Refactor `slovenia-map.tsx` to one marker/popup path and focused hooks/helpers.
7. Split `modeling.py` around dataset, validation, models, intervals, indices, and persistence.
8. Stream CSV members and split typed mapping/conversion from import orchestration.
9. Split the all-purpose analytics repository or extract focused query components.
10. Generate TypeScript transport types from OpenAPI.

## Phase C — Consistency and quality gates

11. Replace duplicated migration lists with a verified migration runner.
12. Add data-vintage metadata and remove hard-coded UI dates/year ranges.
13. Consolidate package management.
14. Expand coverage, contract, cancellation, and performance tests.
15. Align development guidelines with executable CI checks.

---

# 6. Codex working rules

- Audit the current repository again before editing; it may have changed since commit `70b5ec82e964e2cf5061aa6855fb2a6b16bfe610`.
- Do not rewrite working architecture merely to increase the score.
- Make focused changes in dependency order.
- Prefer behavior-preserving refactors with characterization tests.
- For each finding, classify it as:
  - `CONFIRMED`
  - `ALREADY_FIXED`
  - `NOT_REPRODUCIBLE`
  - `ACCEPTED_TECHNICAL_DEBT`
  - `IMPLEMENTED`
  - `DEFERRED`
- Record evidence and changed files in `docs/code-quality-remediation-report.md`.
- Keep raw ETN source data immutable.
- Do not combine methodology corrections and broad cosmetic refactoring in one unreviewable change.
- Preserve API compatibility unless a versioned contract change is approved.
- Run the complete test suite after each phase.

## Required verification commands

```bash
make pipeline-lint
make pipeline-test
make api-vet
make api-test
make web-lint
make web-test
make web-build
git diff --check
```

Also add and run the new commands introduced by this audit, such as frontend type-checking, generated-contract drift checks, static analysis, and targeted coverage.

## Definition of done

- Every P0 item is fixed or explicitly rejected with evidence.
- Code-quality ratings are recalculated from the resulting implementation.
- No duplicate map marker/popup implementation remains.
- Model and importer responsibilities are independently testable.
- Browser preflight works for all supported authenticated routes.
- Production API failures cannot silently display demo market values.
- Strict buildable-land statistics use only ETN codes 1–2 by default across all layers.
- Frontend transport types are reproducibly synchronized with OpenAPI.
- Clean-database migrations and complete tests pass.
- `docs/code-quality-remediation-report.md` lists findings, decisions, tests, and remaining debt.
