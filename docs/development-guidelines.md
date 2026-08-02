# Development guidelines

This monorepo favors conventional tooling and cohesive modules over arbitrary
file-size targets. Split code when a unit has a distinct reason to change,
dependency boundary, or independently testable responsibility.

## Go API

- Keep executable wiring in `cmd/real-estate-api`; keep server implementation in
  `internal` so other modules cannot depend on private application packages.
- Organize packages by capability (`analytics`, `geography`, `identity`,
  `httpapi`), not by generic layers such as `utils`.
- Keep names short and idiomatic: lowercase package names, MixedCaps
  identifiers, `ID` rather than `Id`, and no redundant package-name prefix.
- Accept `context.Context` as the first argument for database and request work.
- Return and wrap errors; handlers translate known domain errors into HTTP
  responses. Do not panic for expected failures.
- Define small interfaces at the consumer boundary. Avoid interfaces that only
  mirror an implementation without enabling substitution.
- Keep SQL visible in repository methods. Bound queries, pagination, map
  extents, connection pools, request bodies, and server timeouts.
- Let `gofmt` own formatting. Every change must pass `go vet` and
  `go test -race ./...`.
- Keep Go tests beside the package as `*_test.go`. This is the native Go test
  convention and allows package-internal or external-package tests as needed.
  Integration tests use the `_integration_test.go` suffix and skip when
  `TEST_DATABASE_URL` is unavailable.
- `internal/httpapi/routes.go` is the route source of truth. The OpenAPI test
  fails if runtime operations and the contract diverge.

## Python data pipeline

- Keep the installable package in `apps/data-pipeline/real_estate_etl` and
  executable behavior behind `real-estate-etl` / `python -m
  real_estate_etl.cli`.
- Modules have one primary responsibility:
  - `api.py`: resilient JGP HTTP transport and atomic downloads;
  - `importer.py`: CSV mapping, type conversion, and transactional COPY;
  - `pipeline.py`: reference synchronization and bounded concurrent download
    orchestration;
  - `geography.py`: GURS RPE boundary ingestion;
  - `modeling.py`: offline statistical model refreshes;
  - `cli.py`: argument parsing and command dispatch;
  - `settings.py` / `models.py`: immutable configuration and shared value
    types.
- Keep database DDL in `database/migrations`, not embedded as table-creation
  logic in Python.
- Use type hints for public functions and dataclasses for data passed across
  modules. Prefer explicit dependencies and pure helpers where practical.
- Never load the complete source dataset into memory. Downloads may run in
  parallel, while database imports remain sequential on their owning
  connection.
- Keep Python tests in `apps/data-pipeline/tests`. A separate test tree is
  conventional with pytest and cleanly separates unit/integration fixtures
  from the installed package.
- Unit tests must not require network or PostgreSQL. Integration tests are
  marked `integration`, use isolated schemas, and require
  `TEST_DATABASE_URL`.
- Every change must pass Ruff and pytest. New behavior needs tests at the
  lowest useful level.

## Repository workflow

Before committing:

```bash
make lint
make test
git diff --check
```

Keep generated data, virtual environments, caches, model artifacts, secrets,
and downloaded ZIP files out of Git. Commit migrations and OpenAPI changes
with the code that consumes them. Prefer focused commits with imperative
messages.

## TypeScript frontend

- Keep pages and layouts in `apps/web/app`, feature components in
  `components/dashboard`, shared primitives in `components/ui`, and API/domain
  types in `lib`.
- Use strict TypeScript, accessible native controls, visible focus states, and
  responsive layouts that keep the map and primary evidence ahead of filters
  on mobile.
- Treat sample size, outlier exclusions, source state, loading, and offline
  fallback as part of the visualization—not incidental metadata.
- Keep charts and analytical map layers code-native. Do not require a
  commercial map token for the baseline dashboard.
- Every change must pass ESLint, Vitest, the Next.js production build, and
  `npm audit`.

## Why the test layouts differ

Go discovers package tests from adjacent `*_test.go` files, so moving Go tests
into a separate root folder would make imports and access needlessly awkward.
Pytest supports either layout; this project keeps Python tests outside the
package because it has functional and PostgreSQL integration tests and because
that layout tests the installed package boundary.
