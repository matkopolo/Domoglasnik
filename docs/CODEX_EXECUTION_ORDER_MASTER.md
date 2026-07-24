# Codex master execution order

## Objective

Execute the repository improvements in this strict order:

1. Statistical and methodological correctness
2. Frontend/API utilisation and contextual analytics
3. General code-quality and maintainability improvements

Do not reorder these phases. Do not begin a later phase while an earlier phase has unresolved correctness failures, failing tests, undocumented blockers, or incomplete required reports.

## Repository

- Repository: `matkopolo/NepremicninskiAgent`
- Branch: use the current working branch
- Re-audit the current repository before editing because it may have changed since the documents were prepared.

## Global rules

- Keep raw ETN source data immutable.
- Preserve all land transactions in the database.
- Use only strict verified-buildable ETN codes 1 and 2 for default land statistics, maps, trends, indices, and model populations.
- Keep code 3 and other excluded land stored and separately accessible.
- Do not duplicate existing implementations.
- Add characterization tests before behavior-preserving refactors.
- Keep OpenAPI, SQL, Go, Python, TypeScript, and frontend behavior synchronized.
- Use separate focused commits for each phase.
- Never hide failing tests or replace live production data with demo values.

---

# Phase 1 - Statistical and methodological improvements

## Read first

1. `docs/ETN_Real_Estate_Analyser_Repository_Specific_Guide_v2.pdf`
2. `docs/CODEX_REPOSITORY_AUDIT_AND_IMPLEMENTATION_v2.md`

These documents are authoritative for Phase 1.

## Required work

- Audit current statistical and ML implementation before editing.
- Create `docs/methodology-implementation-audit.md`.
- Fix statistical correctness and implement confirmed missing methodology.
- Prioritize:
  1. temporal and spatial model validation;
  2. honest prediction-interval calibration;
  3. strict verified-buildable land statistics using codes 1 and 2;
  4. accurate repeat-sales naming or recognized repeat-sales regression;
  5. index vintages, sample metadata, and uncertainty;
  6. missingness, publication thresholds, and near-duplicate improvements.
- Reuse existing analytical facts, outlier logic, anomaly tables, model tables, and API patterns.

## Phase 1 completion gate

Do not start Phase 2 until all of the following are true:

- `docs/methodology-implementation-audit.md` exists and is complete.
- Phase 1 implementation report exists at `docs/methodology-implementation-report.md`.
- Relevant Python, SQL/database, Go API, OpenAPI, and frontend compatibility tests pass.
- Strict buildable-land defaults are tested across database, API, and existing frontend behavior.
- Prediction validation and interval coverage are evaluated honestly.
- Any deferred item includes a concrete reason and data requirement.
- Phase 1 changes are committed in one or more focused commits.

If a blocker prevents completion, document it and stop. Do not skip forward to Phase 2.

---

# Phase 2 - Frontend/API utilisation and contextual analytics

## Read first

1. `docs/CODEX_FRONTEND_API_UTILISATION_AND_CONTEXTUAL_ANALYTICS_v2.pdf`
2. `docs/CODEX_FRONTEND_API_UTILISATION_AND_CONTEXTUAL_ANALYTICS_v2.md`
3. Phase 1 audit and implementation report

Phase 2 must build on the corrected statistical definitions from Phase 1. It must not recreate statistical calculations inconsistently in the browser.

## Required work

- Create `docs/frontend-api-utilisation-audit.md`.
- Classify every API endpoint as used, partially used, intentionally backend-only, analyst-only, operational, or missing from the frontend.
- Refactor the map/dashboard around a contextual statistics panel.
- Support hierarchy:
  - Slovenia;
  - statistical region;
  - municipality;
  - district/local community/settlement where supported;
  - building;
  - transaction/property.
- Integrate valuable existing endpoints and response fields.
- Add backend-computed same-year/property-group comparisons for selected transactions.
- Show comparisons against nearby representative transactions and geography levels with sample size and reliability.
- Do not calculate market benchmarks from only the currently visible map markers.
- Omit unavailable property fields instead of showing ordinary rows such as `Nadstropje: Ni podatka`.
- Treat `positionInBuilding` as real ETN source data, but omit generic values such as `nadstropje` from compact popups.
- Use human-readable codebook descriptions.
- Do not expose private notes or sensitive identifiers without explicit approval.
- Create `docs/frontend-property-field-audit.md`.

## Phase 2 completion gate

Do not start Phase 3 until all of the following are true:

- `docs/frontend-api-utilisation-audit.md` exists.
- `docs/frontend-property-field-audit.md` exists.
- `docs/frontend-contextual-analytics-implementation-report.md` exists.
- All API endpoints are classified, even when intentionally unused.
- Region, municipality, local-area, building, and transaction contexts are implemented or explicitly deferred with evidence.
- Contextual percentage comparisons are backend-computed and tested.
- Missing optional fields are omitted consistently.
- Map/detail requests handle cancellation and stale responses.
- OpenAPI, Go, frontend type checks, frontend tests, and production build pass.
- Phase 2 changes are committed separately from Phase 1 and Phase 3.

If a blocker prevents completion, document it and stop. Do not skip forward to Phase 3.

---

# Phase 3 - General code-quality and maintainability improvements

## Read first

1. `docs/CODE_QUALITY_AUDIT_AND_CODEX_ACTIONS.md`
2. Phase 1 and Phase 2 reports

Phase 3 is last because broad refactoring must happen only after statistical behavior and frontend requirements are stable and covered by tests.

## Required work

- Create `docs/code-quality-remediation-report.md`.
- Re-verify every finding against the current code.
- Classify findings as:
  - `CONFIRMED`
  - `ALREADY_FIXED`
  - `NOT_REPRODUCIBLE`
  - `ACCEPTED_TECHNICAL_DEBT`
  - `IMPLEMENTED`
  - `DEFERRED`
- Fix remaining P0 correctness defects first.
- Refactor only with characterization tests.
- Prioritize:
  1. CORS correctness for authenticated browser routes;
  2. removal of silent production demo fallback;
  3. streaming importer and strict conversion diagnostics;
  4. decomposition of `modeling.py`;
  5. decomposition of `slovenia-map.tsx` if Phase 2 did not already complete it;
  6. focused analytics repositories/query components;
  7. generated TypeScript API types and contract drift checks;
  8. migration runner and executable quality gates.
- Do not undo working Phase 1 or Phase 2 behavior merely to satisfy file-size preferences.

## Phase 3 completion gate

- `docs/code-quality-remediation-report.md` exists.
- Every P0 finding is fixed or rejected with evidence.
- All behavior-preserving refactors have characterization tests.
- Complete lint, test, build, race, contract, and migration checks pass.
- The final report includes remaining technical debt and updated component ratings.
- Phase 3 changes are committed separately.

---

# Required verification

Run project-native commands and any new commands introduced by the work:

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

Also run:

- clean-database migration verification;
- OpenAPI/generated-client drift check;
- frontend type checking, including tests;
- targeted statistical validation and interval-coverage tests;
- relevant query-plan/performance checks for contextual analytics.

# Final response format

At the end, report:

1. Phase 1 changes and tests
2. Phase 2 changes and tests
3. Phase 3 changes and tests
4. Files and migrations changed
5. Deferred items and blockers
6. Remaining risks
7. Final code-quality ratings

Do not claim completion for a phase whose completion gate is not satisfied.
