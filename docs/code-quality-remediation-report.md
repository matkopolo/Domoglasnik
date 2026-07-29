# Phase 3 code-quality remediation report

## Scope

Every earlier finding was re-checked after Phases 1 and 2. Characterization
tests preceded behavior-preserving extraction. Phase 1 populations,
strict-land rules, outlier definitions, intervals, and indices are unchanged;
Phase 2 frontend behavior is preserved.

## Finding re-audit

| Finding | Status | Evidence / disposition |
|---|---|---|
| DATA-P0 archive streaming | IMPLEMENTED | ZIP members use `ZipFile.open`/`TextIOWrapper`, incremental `COPY`, streaming checksums, and bounded codebook batches. A test disables `ZipFile.read`. |
| DATA-P0 strict conversion | IMPLEMENTED | `conversion.py` uses `Decimal`, strict booleans, sentinel handling, and contextual `SourceConversionError`; table-driven tests cover valid and malformed data. |
| DATA-P1 mapping separation | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Conversion left importer. Stable BUY/RENT registries stay together until another source product creates a real reason to extract `mappings.py`. |
| DATA-P1 modeling decomposition | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Feature construction and pure validation/metric policy moved to `model_features.py` and `model_validation.py` behind the compatibility façade. Persistence/index orchestration remains cohesive. |
| DATA-P1 model tests | ALREADY_FIXED / IMPLEMENTED | Phase 1 supplied deterministic split, interval, percentile, run, vintage, and statistical tests; they characterize this extraction. Cross-fit fault injection remains debt. |
| DATA-P2 orchestration/logging | ACCEPTED_TECHNICAL_DEBT | Typed outcomes and bounded orchestration exist. Structured worker logging belongs with the Phase 4 machine consumer. |
| API-P0 authenticated CORS | IMPLEMENTED | Browser write methods, Authorization, and required `Vary` fields are covered by table-driven preflight tests. |
| API-P1 repository split | ACCEPTED_TECHNICAL_DEBT | Capability SQL is already split into focused files and consumers use narrow interfaces. Splitting the shared pool-owning concrete type now would only multiply constructors. |
| API-P1 SQL/geography fragments | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Central allow-listed geography columns cover every level/unknown in tests; opaque `OR true` clauses are gone. Larger aggregate deduplication is deferred to keep SQL reviewable. |
| API-P1 contract validation | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Brittle fixed path count removed; parsing, route parity, and unique operations remain. Generated frontend schema/drift added. Runtime response-schema validation remains debt. |
| API-P2 middleware/observability | IMPLEMENTED / DEFERRED | First response status is retained and tested. Metrics/rate limiting are operational features outside this refactor. |
| API-P2 domain synchronization | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | `/meta` and existing consistency tests remain; generated TypeScript reduces transport drift. SQL checks remain migration-authoritative. |
| API-P2 migration duplication | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | `db-up` delegates to one ordered migration loop; clean CI applies all migrations. A checksum ledger is Phase 4 debt. |
| WEB-P0 demo fallback | ALREADY_FIXED | Phase 2 made demo explicit and tested live/error/abort/stale/demo state. |
| WEB-P0 stale async work | ALREADY_FIXED | Phase 2 added owned cancellation/generation guards and rapid-change coverage. |
| WEB-P1 map decomposition | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Popup/amount construction moved to a pure tested module. MapLibre lifecycle extraction awaits an adapter-level characterization harness. |
| WEB-P1 generated API types | IMPLEMENTED | Pinned `openapi-typescript`, checked-in schema, Make command, and CI drift gate added. UI view models stay separate. |
| WEB-P1 hard-coded dates | ALREADY_FIXED | Phase 2 uses API vintage and available-year metadata. |
| WEB-P2 frontend depth | ALREADY_FIXED / IMPLEMENTED | Phase 2 interaction tests remain; Phase 3 adds popup formatting and test-source compilation. Full MapLibre/a11y automation remains debt. |
| WEB-P2 package management | ACCEPTED_TECHNICAL_DEBT | `apps/web` is authoritative; root remains a deployment wrapper until a hosting target is selected. |
| WEB-P2 test typecheck | IMPLEMENTED | `tsconfig.test.json`, npm, Make, and CI gates compile application and tests; the new gate found three existing typing defects, now fixed. |
| CONSISTENCY-P0 strict land | ALREADY_FIXED | Phase 1 traces codes 1–2 through facts, API, maps, frontend, models, contract, and tests. |
| CONSISTENCY-P1 transport duplication | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | Generated schema/drift gate exists; UI-only reshaping models remain intentional. |
| CONSISTENCY-P1 executable guidelines | IMPLEMENTED / DEFERRED | Streaming, strict conversion, typecheck, contract drift, migrations, and builds are executable. Production dependency audit is clean; development-only advisories require a separate tooling-upgrade decision. |
| CONSISTENCY-P1 shared gates | IMPLEMENTED / ACCEPTED_TECHNICAL_DEBT | CI covers Python lint/tests, clean migrations, Go fmt/vet/build/race, and web lint/typecheck/contract drift/tests/build. Coverage thresholds, staticcheck, govulncheck, and performance budgets remain named debt. |

## Characterization-first evidence

- Import succeeds when `ZipFile.read` is made unusable.
- Conversion tests lock valid, blank, sentinel, exact decimal, and malformed
  source behavior.
- Existing deterministic model tests continued through the compatibility
  façade before extraction.
- Popup tests lock title, €/m², area, total price, and median preference.
- CORS and response-recorder tests specify middleware behavior directly.

## Completion gate and verification

The completion run covers Ruff and all pytest tests (including PostgreSQL);
Go formatting, vet, build, and race-enabled unit/integration/OpenAPI tests;
ESLint, production-and-test TypeScript, Vitest, generated-contract drift, and
the Next.js production build; clean ordered migrations; upgrade migration
rerun; and `git diff --check`. Phase 1 statistical validation and Phase 2
frontend behavior tests are included in those suites.

Final results: Ruff clean; pytest 45 passed; Go vet clean and all
race-enabled packages passed; Vitest 27 passed; frontend application/test
typecheck clean; OpenAPI generation produced no drift; clean migrations 001–022
passed in a disposable database; API and web images built; `npm audit
--omit=dev` reported zero vulnerabilities; and `git diff --check` passed. The
full development dependency tree currently reports 11 high advisories, so a
non-breaking toolchain upgrade remains explicit debt rather than being hidden
by an automatic forced upgrade.

## Updated ratings

| Component | Before | After |
|---|---:|---:|
| Python data pipeline | 7.2 | **8.3** |
| Go API | 7.7 | **8.4** |
| Next.js web | 6.6 | **8.0** |
| Cross-layer consistency | 7.1 | **8.2** |
| Whole repository | 7.2 | **8.2** |

No P0 finding remains open. Remaining debt is contained and listed above.

## Phase 4 handoff

Containerize the pipeline as a one-shot/scheduled worker; make Docker Compose
the normal Windows/macOS/Linux entrypoint; add an explicitly destructive
reset-and-repopulate command; and run incremental source discovery every two
months followed by transactional analytics/model refresh. Use
`source_files`, checksums, and refresh state instead of downloading blindly,
and never publish partially refreshed derived data.
