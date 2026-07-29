# Phase 1 methodology implementation report

Date: 2026-07-29

## Outcome

Phase 1 is complete. Statistical definitions were corrected without changing
immutable ETN source rows, and no Phase 2 frontend implementation or Phase 3
general refactoring was performed.

The pre-change findings and evidence are in
`docs/methodology-implementation-audit.md`.

## Implemented changes

### Honest predictive validation and intervals

The existing property-group log-Ridge and Isolation Forest approach was
retained. Its evaluation was corrected:

- training uses dates before the calibration year;
- the preceding complete calendar year is the disjoint calibration set;
- the latest complete calendar year is the untouched temporal test set;
- sparse datasets have a deterministic chronological fallback;
- a separate deterministic municipality-blocked holdout measures spatial
  generalization;
- the train-only local median is reported as a baseline;
- reported metrics include MAE, median APE, log RMSE, log R², sample, date
  ranges, interval coverage, and interval width;
- prediction half-widths use a finite-sample split-conformal absolute-log
  residual quantile;
- only held-out residuals receive residual-anomaly ranks.

The full-data refresh trained through 2023, calibrated on 2024, and tested on
2025:

| Kind | Eligible model rows | Test rows | Median APE | Log R² | 90% interval coverage |
|---|---:|---:|---:|---:|---:|
| Sale | 283,366 | 13,617 | 47.59% | 0.4830 | 88.35% |
| Rent | 307,668 | 3,614 | 38.02% | 0.1624 | 88.16% |

Coverage is close to, but below, the nominal 90%; it is reported rather than
corrected cosmetically. Several subgroup and municipality-holdout R² values
remain weak or negative. Valuations therefore remain statistical estimates,
not decision-grade appraisals.

### Strict buildable-land population

`analytics.land_transaction_facts` now classifies every land-only contract at
transaction grain and keeps contract count distinct from parcel count.

The populated database contains:

| Population | Contracts |
|---|---:|
| All retained land | 285,102 |
| Broad codes 1–3 only | 80,598 |
| Strict codes 1–2 only | 19,856 |

Strict means every parcel in the contract is code 1 and/or 2. A contract that
mixes a strict parcel with code 3 or another class is not strict. House or
apartment packages with attached land keep their existing primary property
classification.

Sale statistics, trends, distributions, maps, outlier lists, and model
populations now use `analytics.sale_facts_strict_land` by default. The common
API filter supports:

- `landScope=strict` — default, codes 1–2 only;
- `landScope=broad` — opt-in, codes 1–3 only;
- `landScope=all` — explicit unrestricted audit access.

Existing frontend requests send `propertyGroup=land` without `landScope`, so
they receive the corrected strict default without frontend changes. Database,
repository integration, HTTP default/validation, OpenAPI, and existing
frontend tests cover this behavior.

### Accurate index methods and append-only vintages

The former `repeat_sales` series is now `matched_pair_median`. It compounds
median annualized log returns from consecutive eligible sales of the same
cadastral building part; it no longer claims to be a recognized weighted
repeat-sales regression.

`analytics.index_calculation_runs` records append-only calculation vintages,
data ranges, methodology versions, and parameters.
`analytics.current_market_indices` exposes the latest vintage while
`analytics.market_indices` retains prior calculations.

Index rows now distinguish:

- effective-period observed sample;
- hedonic reference-basket sample;
- lower and upper index values;
- nominal coverage and uncertainty method;
- publication status and suppression reason;
- calculation and model runs.

Hedonic uncertainty uses 100 deterministic reference-basket bootstrap
replicates. Matched-pair median series expose empirical pair-return uncertainty
bands. Cells below 30 observations/pairs remain stored but are marked
unpublishable.

### Missingness, publication, and duplicate review

- `analytics.publication_policy` centralizes the descriptive/index minimum of
  30 and its policy version.
- Overview, time-series, rental, distribution, and index API responses expose
  `isPublishable` and `suppressionReason`; counts are not hidden.
- `analytics.missingness_profiles` contains 3,240 national and 226,680
  municipality cells by transaction kind, property group, contract year, and
  field. Target values are never imputed.
- `analytics.near_duplicate_candidates` records cross-transaction
  same-building-part candidates within 30 days, 2% area, and 2% amount,
  including distances, score, and rule version.
- The current database produced 497,429 near-duplicate review candidates.
  They update the review signal but never remove or automatically invalidate a
  transaction.
- Candidate generation starts from one-row-per-transaction facts, so a
  multi-component package cannot be mistaken for duplicate rows inside the
  same transaction.

## Migrations and operational order

New migrations:

- `020_land_methodology.sql`;
- `021_index_vintages.sql`;
- `022_quality_observability.sql`.

`make db-migrate` includes all three. `make analytics-refresh` now refreshes
sale facts, land facts, outliers, rental facts, anomaly signals, near
duplicates, missingness, and the matched-pair median series in dependency
order. `make models-refresh` appends model and hedonic-index vintages.

The migrations were applied twice to the populated database and once, in
filename order, to an empty temporary database. The temporary database was
removed after all expected Phase 1 relations were verified.

## Verification

Passed:

- Python Ruff;
- Python unit and PostgreSQL integration tests: 31 passed;
- targeted split/leakage/conformal tests: 7 passed;
- Go formatting and vet;
- Go race/unit/integration/API/OpenAPI tests: all packages passed;
- clean-database migration verification;
- populated-database migration idempotency;
- full sale/rent model and index refresh;
- frontend ESLint;
- frontend compatibility tests: 7 passed;
- frontend production Docker build;
- `git diff --check`.

The Python suite emits two existing warnings because the repository has not
registered its `integration` pytest marker. That is test-configuration cleanup
and is intentionally left to Phase 3 rather than mixed into this methodology
phase.

## Explicit deferrals

### Weighted repeat-sales regression

Not implemented because the inaccurate claim was removed by choosing the
master-approved alternative: accurate matched-pair median naming. Before
adding a Case-Shiller-style weighted regression, the project needs a pair
diagnostic specification and adequate stable pair counts per requested
geography/period, including holding-period variance weights and renovation or
major-change exclusions. It must be introduced as a separate method, not by
renaming the current series again.

### Moran's I

Deferred until a spatial weights definition is approved (municipality
adjacency, distance band, or k-nearest neighbors) and residual diagnostics
have a frozen evaluation population. Current municipality-blocked metrics are
the honest prerequisite and already show material spatial generalization
weakness.

### Automatic duplicate rejection

Intentionally not implemented. A near match can be a renewal, correction,
partial-interest sale, or legitimate repeat transaction. A labelled human
review set is required before thresholds can support any stronger policy.

## Remaining methodological risks

- Prediction performance is weak for several subgroups; intervals are broad
  and empirical coverage is not exactly nominal.
- Current ETN centroids and current administrative polygons do not reconstruct
  historical legal geography.
- The strict land definition is verified ETN readiness classification, not an
  independent current planning-permission or zoning determination.
- Matched-pair empirical bands describe pair-return dispersion; they are not
  bootstrap confidence intervals for a weighted repeat-sales estimator.
- Near-duplicate candidate volume is high and needs labelled review before
  precision/recall can be calibrated.

## Phase 1 completion gate

- [x] Audit exists and was written before production changes.
- [x] Implementation report exists.
- [x] Relevant Python, SQL/database, Go, OpenAPI, and frontend compatibility
  checks pass.
- [x] Strict codes 1–2 defaults are tested across database, API, and existing
  frontend behavior.
- [x] Temporal/spatial validation and interval coverage are evaluated
  honestly.
- [x] Deferrals include concrete reasons and required data/specifications.
- [x] Phase 1 changes are committed in focused commits.
