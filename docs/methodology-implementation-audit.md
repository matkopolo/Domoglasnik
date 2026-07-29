# Phase 1 methodology implementation audit

Date: 2026-07-29

## Scope and authority

This audit was completed before changing production code. It covers Phase 1
only, using:

- `CODEX_EXECUTION_ORDER_MASTER.md`;
- `ETN_Real_Estate_Analyser_Repository_Specific_Guide_v2.pdf`;
- `CODEX_REPOSITORY_AUDIT_AND_IMPLEMENTATION_v2.md`;
- the current SQL migrations, Python pipeline, Go API, OpenAPI contract, tests,
  documentation, and the populated local PostgreSQL/PostGIS database.

Where the older guide describes codes 1–3 as a broad buildable-land category,
the master file takes precedence: default statistics, maps, trends, indices,
and model populations must use only verified-buildable ETN codes 1 and 2.
Code 3 and every other land transaction must remain stored and separately
accessible.

## Existing implementation that must be reused

| Area | Current implementation | Audit conclusion |
|---|---|---|
| Source retention | Immutable transaction, building-part, land-parcel, rental, source-file, and codebook tables | Retain; no raw-row deletion or mutation is needed |
| Sale facts | `analytics.sale_facts`, one row per transaction, with package classification, price derivation, geography, and quality reasons | Reuse as the general sale fact and add a purpose-specific land fact |
| Rental facts | `analytics.rental_facts` with normalized monthly rent and quality rules | Reuse |
| Robust outliers | Versioned IQR/MAD evaluations and `representative_eligible` | Reuse; do not silently delete anomalies |
| Anomaly review | `analytics.anomaly_signals` with persistent review state and explainable component scores | Extend duplicate signals, preserving review state |
| Models | Per-property-group log-Ridge, cross-fitted residual ranking, Isolation Forest, model runs, and transaction predictions | Correct validation and calibration rather than replacing the model family |
| Indices | Hedonic monthly index and same-building-part matched-pair annual growth index | Preserve both, correct naming and revision semantics |
| API | Shared `StatisticsFilter` used by statistics, maps, distributions, and outliers | Extend this common filter so land semantics stay consistent |

## Evidence from the populated database

The ETN codebook contains ten land types. Codes 1 and 2 mean respectively:
land with a building permit, and buildable serviced land. Code 3 includes land
that *is or will be* buildable and may be only partly serviced or unserviced,
so it is not part of the strict default population.

The local database contains:

| ETN code | Parcel rows | Distinct transactions |
|---:|---:|---:|
| 1 | 46,861 | 27,265 |
| 2 | 11,881 | 7,068 |
| 3 | 138,926 | 83,896 |
| 4–10 | 826,110 | varies by class |
| missing | 21 | 8 |

`analytics.sale_facts` currently contains 285,102 land-classified
transactions, of which 163,822 are representative under the general quality
and outlier rules. The API currently treats all of these as the same
`propertyGroup=land` population. That violates the master default and can
materially distort counts and price/m² summaries.

## Findings and required Phase 1 changes

### 1. Temporal and spatial model validation — confirmed gap

`modeling.py` uses shuffled five-fold `KFold` within each property group.
Transactions from later dates can therefore train predictions for earlier
dates, and nearby municipalities can occur on both sides of a fold. Current
metrics are cross-fold metrics but are neither out-of-time nor spatially
blocked.

Required:

- deterministic latest-complete-period temporal evaluation;
- deterministic municipality-blocked spatial evaluation;
- a train-only local-median baseline;
- persisted split ranges/grouping and MAE, median APE, log RMSE, log R², and
  subgroup metrics;
- explicit small-sample behavior and leakage tests.

### 2. Prediction intervals — confirmed correctness defect

The 90th percentile of shuffled-fold absolute log residuals is used as a
single property-group half-width. Stored intervals are then centered on
predictions from a final model fitted on every row, and reported coverage is
computed on those in-sample centers. The resulting coverage is not an honest
validation estimate.

Required:

- reserve calibration rows that are disjoint from model fitting and final
  temporal test rows;
- derive interval widths only from calibration residuals;
- report empirical coverage and mean interval width only on the untouched
  temporal test set;
- label persisted interval method and nominal coverage.

### 3. Strict buildable-land defaults — confirmed correctness defect

The raw data is retained correctly, but `sale_facts` only stores the broad
`land` property group. Statistics, trends, maps, distributions, outliers, and
the sale-model query do not inspect land type. The frontend sends
`propertyGroup=land`, so existing UI behavior inherits the unrestricted
backend population.

Required:

- a transaction-grain land analytical fact retaining every land sale;
- parcel count, distinct ETN codes, readiness class, strict eligibility,
  comparable area/price, and quality metadata;
- default `propertyGroup=land` selection restricted to transactions whose
  land parcels are exclusively codes 1 and/or 2;
- explicit `strict`, `broad`, and `all` scopes, where broad is codes 1–3 and
  all is audit access;
- contract and parcel counts kept distinct;
- database, API, model-query, and existing frontend compatibility tests.

Land attached to a house or apartment remains part of that package and is not
reclassified as a land-only analytical transaction.

### 4. Repeat-sales method naming — confirmed defect

`refresh_repeat_sales_indices()` pairs consecutive transactions for the same
cadastral building part and compounds median annualized log returns. This is a
valid matched-pair median growth series, but it is not a recognized weighted
repeat-sales regression and must not be named `repeat_sales`.

Required:

- rename the method to `matched_pair_median`;
- preserve pair-quality rules and expose the method definition;
- do not claim a Case-Shiller or weighted repeat-sales regression.

A recognized repeat-sales regression is deferred until pair diagnostics show
adequate sample depth by geography and period; the current local series only
publishes property-group/year cells with at least 30 pairs.

### 5. Index vintages, samples, and uncertainty — confirmed gaps

Both index refresh paths delete current rows before inserting replacements.
There is no append-only calculation vintage, revision identifier, published
flag, uncertainty interval, or effective-period observation count distinct
from the hedonic reference sample. Hedonic `sample_size` currently records
reference-basket size, not period observations.

Required:

- append-only index calculation runs/vintages and a current-index view;
- per-period observed sample size and reference sample size;
- deterministic bootstrap uncertainty where supported;
- minimum-sample publication state and suppression reason;
- API metadata for vintage, interval, and publication status.

### 6. Missingness, publication thresholds, and near duplicates — confirmed gaps

Quality reasons expose row-level missing inputs, but no aggregate missingness
profile exists by fact kind, field, property group, geography, source year,
and contract period. Descriptive API cells are returned even for samples of
one. Duplicate detection only flags exact analytical matches.

Required:

- refreshable missingness profiles with numerator, denominator, rate, period,
  geography, and data vintage;
- shared minimum sample threshold and an `isPublishable`/suppression signal in
  descriptive results without hiding counts;
- explainable near-duplicate candidates using identity, date, area, and amount
  tolerances;
- tests proving a multi-component package is not itself treated as duplicate
  transaction rows.

Global Moran's I is deliberately deferred. It requires a frozen, honestly
validated residual set and a documented spatial weights matrix. Adding it
before those prerequisites would create a precise-looking but unstable
diagnostic.

## Compatibility and risk notes

- Raw ETN tables remain unchanged and all land rows remain queryable.
- Existing `propertyGroup=land` frontend requests must become strict through
  the API default without requiring frontend Phase 2 work.
- Counts may decrease for default land views; this is an intentional
  definition correction, not data loss.
- Migration refresh functions must remain idempotent and safe on a populated
  database.
- Existing clients that request `method=repeat_sales` need a clear validation
  error or migration path; silently preserving the incorrect label is not
  acceptable.
- Performance-sensitive land filtering needs indexed transaction-grain facts,
  not repeated parcel scans in every endpoint.

## Phase 1 implementation plan

1. Add the land fact, shared land-scope SQL predicate, missingness profiles,
   publication policy, near-duplicate candidates, and append-only index
   vintage schema in focused migrations.
2. Apply strict land scope through the common Go analytics filter, expose
   methodology metadata in relevant responses, and synchronize OpenAPI.
3. Replace shuffled evaluation with deterministic temporal and spatial
   validation and split-conformal calibration; record honest metrics and
   interval metadata.
4. Rename the matched-pair method and make both index refreshes append-only
   with observed samples, uncertainty, and publication state.
5. Add SQL/integration/unit/contract/frontend-compatibility tests, validate
   migrations from a clean database, and document operational refresh order.

No frontend redesign and no general code-quality refactoring are in scope.
