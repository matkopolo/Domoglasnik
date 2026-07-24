# Codex repository audit and implementation task - ETN Real Estate Analyser

## Repository context

- Repository: `matkopolo/NepremicninskiAgent`
- Branch: `main`
- Repository state inspected for this guide: commit `70b5ec82e964e2cf5061aa6855fb2a6b16bfe610`
- Methodology reference: `ETN_Real_Estate_Analyser_Repository_Specific_Guide_v2.pdf`

This is not a greenfield implementation. The repository already contains transaction-grain analytical facts, robust outlier evaluation, a layered anomaly queue, hedonic Ridge models, Isolation Forest scoring, valuation ranges, hedonic indices, repeat-transaction matching, geographic assignment, a Go API, and tests.

Do not create duplicate implementations. Audit first and modify only confirmed gaps.

## Mandatory land-retention and visibility rule

- Keep **every land sale** in the source database and analytical database. Never delete or discard non-buildable, agricultural, forest, mixed, or unknown land transactions.
- Classify every land transaction by buildability/readiness.
- The default user-facing **building plot / buildable land** statistics, maps, filters, and lists must include only land on which construction is possible under the verified ETN classification (the agreed buildable classes, currently codes `1`, `2`, and `3`).
- Non-buildable, mixed, and unknown land remains stored and auditable, but is excluded from the default building-plot view.
- Expose non-buildable land only through an explicit audit/raw-data or opt-in filter. Never silently mix it into building-plot prices, counts, medians, maps, or trends.
- Report buildable-land contract counts and parcel counts separately.

## Mandatory first phase - no production edits

1. Read `README.md`, `apps/api/README.md`, `apps/api/docs/backend-api-plan.md`, `apps/api/docs/anomaly-methodology.md`, `database/documentation/data-dictionary.md`, `Makefile`, all migrations, and applicable tests.
2. Inspect at minimum:
   - `database/migrations/010_sale_package_classification.sql`
   - `database/migrations/011_sale_facts.sql`
   - `database/migrations/012_sale_outliers.sql`
   - `database/migrations/014_rental_facts.sql`
   - `database/migrations/015_rental_outliers.sql`
   - `database/migrations/016_anomaly_signals.sql`
   - `database/migrations/018_valuation_models.sql`
   - `database/migrations/019_market_indices.sql`
   - `apps/data-pipeline/real_estate_etl/modeling.py`
   - `apps/data-pipeline/tests/test_modeling.py`
   - `apps/api/openapi/openapi.yaml`
   - the Go handlers, repositories, SQL queries, and tests used by statistics, anomalies, indices, and valuations.
3. Create `docs/methodology-implementation-audit.md` with one row per method and these statuses:
   - `COMPLETE`
   - `PARTIAL`
   - `PRESENT_BUT_UNUSED`
   - `PRESENT_BUT_UNTESTED`
   - `MISSING`
   - `DEFERRED_BY_DATA`
   - `NOT_APPLICABLE`
4. For every row, record evidence, tests, methodological gaps, and planned action.
5. Present the audit and implementation plan before changing production code.

## High-confidence findings to verify

### Already present and must be reused

- Contract-grain `analytics.sale_facts` and `analytics.rental_facts`.
- Package classification and price/rent derivation metadata.
- PostGIS geography assignment.
- IQR and MAD comparable-cohort outlier evaluation.
- Non-destructive anomaly signals and human review state.
- Cross-fitted Ridge hedonic residuals.
- Isolation Forest as a secondary signal.
- Persisted model runs, predictions, ranges, and market indices.
- Repeat-transaction matching by cadastral building-part identity.
- API endpoints for statistics, distributions, outliers, anomalies, indices, valuations, maps, and building history.

### Likely partial or missing work

Verify each item against the repository before implementing it:

1. Predictive validation currently uses shuffled `KFold`; implement temporal and spatially blocked evaluation.
2. Persisted valuation intervals appear to be centred on in-sample final-model predictions; replace or calibrate them with honest out-of-fold/out-of-time residuals.
3. Add explicit baseline comparisons: local median price/rent per square metre and transparent hedonic regression.
4. Add subgroup metrics by property group, geography, time, area band, and price band.
5. Add missingness indicators and missing-data coverage reports; never impute target values.
6. Implement buildable-land analysis for ETN land codes 1, 2, and 3, with a stricter 1-2 category and separate readiness reporting.
7. Keep buildable-land analysis at contract grain and report contracts separately from parcels.
8. The current `repeat_sales` index is a median of annualized matched-pair returns, not a Case-Shiller weighted repeat-sales regression. Either rename it accurately or implement the recognized regression method beside it.
9. Preserve index vintages/revisions instead of only deleting and replacing the current index rows.
10. Add bootstrap uncertainty and publication thresholds for descriptive statistics and indices.
11. Add near-duplicate detection; exact analytical duplicate candidates already exist.
12. Add spatially blocked residual diagnostics and Moran's I only after the validation dataset is correct.
13. Add a nonlinear ML benchmark only after the transparent baseline and validation framework are correct. Prefer a controlled benchmark and avoid adding a dependency without measured benefit.
14. Audit `marketability`, deed/transaction type, ownership share, and codebook semantics. Do not hard-code a final/provisional interpretation until verified from the imported codebook.
15. Add segment-level gross rental-yield estimates only as modelled cohort comparisons, never as a matched property or net investment return.

## Priority work packages

### WP1 - validation and honest uncertainty

- Replace shuffled random validation for published model metrics with:
  - latest-complete-period temporal holdout;
  - rolling/forward-chaining evaluation where data history permits;
  - spatially blocked holdout using municipality, statistical region, or a documented coarse spatial cell.
- Keep cross-fitting for anomaly residuals, but ensure a record is not scored by future observations when the score is presented historically.
- Store split method, train/validation date ranges, spatial grouping, and metric version in `analytics.model_runs.feature_schema` or a new structured field.
- Report MAE, median absolute percentage error where defined, RMSE on log target, log R-squared, empirical interval coverage, and subgroup metrics.
- Add tests for leakage, deterministic splits, empty/small groups, and interval coverage calculations.

### WP2 - valuation baselines and ranges

- Implement and persist a local median-per-square-metre baseline.
- Keep the existing Ridge hedonic model as the transparent benchmark.
- Use out-of-fold or conformal calibration residuals for prediction ranges.
- Do not calculate range coverage from in-sample final-model predictions.
- Add a nonlinear benchmark only after baselines pass. CatBoost is suitable for mixed categorical/numeric data, but adding it must be justified by out-of-time and out-of-area improvement.
- Keep predictions statistical and clearly separate from official GURS valuations.

### WP3 - buildable land / building plots

Before adding a migration, verify that no migration newer than `019_market_indices.sql` already implements this.

Recommended analytical object: `analytics.land_transaction_facts` or an equivalent project-consistent name.

Required fields:

- `transaction_id`
- `contract_date`
- `dataset_year`
- `parcel_count`
- `land_type_codes`
- `buildability_class`
- `total_area`
- `analytical_price`
- `price_per_sqm`
- `price_derivation`
- geography IDs and representative location
- `is_quality_eligible`
- `quality_reasons`

Required categories:

- `building_permit` - code 1
- `serviced_buildable` - code 2
- `partly_serviced_or_future_buildable` - code 3
- `buildable_broad` - all included parcels are codes 1-3
- `buildable_strict` - all included parcels are codes 1-2
- mixed/non-comparable land packages remain separately visible

Rules:

- Keep all land sales in the database, including non-buildable, agricultural, forest, mixed, and unknown land.
- Prefer land-only transactions for clean plot statistics.
- Aggregate compatible parcels before calculating price per square metre.
- Never copy the contract price onto every parcel.
- Report unique transaction count and parcel count separately.
- Keep codes 1, 2, and 3 separately visible because readiness materially affects value.
- Default public building-plot statistics, maps, filters, and lists to buildable classes only (currently ETN codes 1-3).
- Exclude non-buildable, mixed, and unknown land from the default building-plot view, but retain it for audit/raw-data access or an explicit opt-in filter.
- Add API filters/metadata without silently changing the meaning of the existing `property_group='land'` category.
- Add SQL and API integration tests proving that non-buildable land remains stored but is absent from default building-plot results.
- Add SQL and API integration tests for one-parcel, multi-parcel, mixed-land, building-plus-land, missing-price, and missing-area cases.

### WP4 - robust statistics, missingness, and publication policy

- Verify current percentile, histogram, median, cleaned mean, and sample-size behavior.
- Add configurable publication thresholds and unstable-cell flags.
- Add deterministic/bootstrap confidence intervals where computationally appropriate.
- Add missingness profiles by field, property group, geography, source year, and contract period.
- Store and expose analysis population, eligible count, excluded count, date basis, derivation, rule/model version, and data vintage.

### WP5 - index methodology

- Keep a simple stratified median index as a composition-sensitive reference.
- Correct and validate the constant-quality hedonic index with honest period coverage and index-run metadata.
- Store observed sample size for each index period, not only reference-sample size.
- Use a stable base period and preserve index vintages/revisions.
- For repeat transactions:
  - retain the existing matched-pair series under an accurate method name, or
  - implement a recognized weighted repeat-sales regression with pair-quality rules and diagnostics.
- Do not enable bubble or cycle claims until a long, stable quarterly series and macroeconomic variables exist.

### WP6 - anomaly improvements

- Reuse `analytics.anomaly_signals` and its review state.
- Add near-duplicate candidates with explainable distance/date/amount/identity criteria.
- Keep model residual and Isolation Forest signals secondary; they must not independently delete or exclude source rows.
- Calibrate thresholds from reviewed labels when enough labels exist.
- Preserve model/rule versions and review decisions across refreshes.
- Add tests proving that multi-component packages are not collapsed as duplicates.

### WP7 - rental yield

- Calculate only a market-segment gross yield estimate:
  `12 * representative monthly rent / representative sale price`.
- Match cohorts by geography, property group, size, age, and period.
- Return sale and rental sample sizes separately.
- Label outputs as modelled segment estimates.
- Do not call the result net yield without vacancy, maintenance, operating expenses, taxes, and financing.

## Deferred methods

Mark as `DEFERRED_BY_DATA` unless the repository has sufficient evidence:

- hierarchical/partial-pooling production models;
- GWR/MGWR production outputs;
- ML chained price indices with fewer than eight complete stable quarters;
- bubble tests, structural-break tests, and cycle dating without a long quarterly history and fundamentals;
- causal monetary-policy analysis without an identified research design.

## Definition of done

- The audit maps every methodology item to actual files and tests.
- Existing implementations are reused rather than duplicated.
- Temporal and spatial leakage tests pass.
- Prediction ranges have honest held-out coverage.
- Building-plot statistics distinguish ETN codes 1, 2, and 3 and retain contract grain.
- Every land sale remains stored, while default user-facing building-plot results contain only verified buildable classes.
- Index methods are named accurately and preserve vintages.
- Every published result reports sample and methodology metadata.
- Raw ETN data remains immutable.
- All relevant Python, SQL/database, Go API, OpenAPI, and frontend tests pass.
- The final report lists changed, reused, deliberately unchanged, deferred, and blocked components.

## Commands to verify

Use project-native commands and adapt only when the repository has changed:

```bash
make pipeline-lint
make pipeline-test
make api-vet
make api-test
make web-lint
make web-test
make analytics-refresh
make models-refresh WORKERS=6
```

Do not run destructive operations against a production database. Use the documented local/test database.
