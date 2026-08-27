# Valuation model optimization report

This is the measured snapshot of the optimization run identified below, not a
live report regenerated on every scheduled import. Current operational rules
are documented in `docs/statistical-methodology.md`; exact results for a newer
training run are persisted in `analytics.model_runs`.

Run date: 2026-08-03  
Code base commit: `9783ba2981418bd7bfc7bb72f4adc87ff5544aff` (the checked-out HEAD before this change set)  
Random seed: `20260723`

## Outcome

The sale and rental hedonic models now use leakage-safe expanding-window selection. The latest complete calendar year, 2025, is reserved and evaluated once after selection. The current calendar year (2026) is excluded from all validation and is used only by the final production fit. Preprocessing is owned by each scikit-learn pipeline and is fitted on that fold's training rows.

The selected configurations reduced the persisted overall untouched-holdout MAE from the preceding v2 run's €1,037.50/m² to €807.84/m² for sales and from €4.92/m² to €4.06/m² for rentals. This comparison demonstrates better results on the respective 2025 holdouts, but is not a controlled ablation because v3 also removes the higher-ID member of persisted near-duplicate pairs. It must not be interpreted as accuracy outside the eligible ETN population.

## Existing baseline and preserved population

The previous configuration was a separate `Ridge(alpha=2.0)` log-price/rent-per-m² model per property group, one 2025 test year with 2024 conformal calibration, median imputation, standard scaling, one-hot encoding, and a separate deterministic Isolation Forest.

V3 preserves quality eligibility, separate sale/rental populations, property-group separation, raw ETN storage, and strict sale-land population rules. Default land training includes only transactions whose complete parcel population contains verified-buildable ETN codes 1 and/or 2. Code 3, mixed, unknown, and non-buildable land remain outside the default population. A transaction is already one fact row; additionally, the higher transaction ID in every persisted near-duplicate pair is excluded so duplicate-like contracts cannot inflate temporal folds.

## Data and temporal folds

Minimum sizes are configurable through `MODEL_MIN_TRAINING_ROWS`, `MODEL_MIN_CALIBRATION_ROWS`, and `MODEL_MIN_VALIDATION_ROWS`; defaults are 100, 30, and 30. Invalid folds are stored with an explicit reason. All dates below are calendar years.

| Kind/group | Production rows | Selection validation years | Calibration years | Skipped | Final holdout |
|---|---:|---|---|---:|---:|
| Sale apartment | 153,664 | 2009–2024 | 2008–2023 | 0 | 2025 |
| Sale commercial | 25,552 | 2009–2024 | 2008–2023 | 0 | 2025 |
| Sale garage | 16,925 | 2009–2024 | 2008–2023 | 0 | 2025 |
| Sale house | 62,144 | 2009–2024 | 2008–2023 | 0 | 2025 |
| Sale land | 14,048 | 2012–2024 | 2011–2023 | 3 insufficient early folds | 2025 |
| Rent apartment | 168,326 | 2015–2024 | 2014–2023 | 0 | 2025 |
| Rent commercial | 45,675 | 2015–2024 | 2014–2023 | 0 | 2025 |
| Rent garage | 9,738 | 2015–2024 | 2014–2023 | 0 | 2025 |
| Rent house | 14,212 | 2015–2024 | 2014–2023 | 0 | 2025 |
| Rent room | 16,816 | 2015–2024 | 2014–2023 | 0 | 2025 |

Every fold trains through year N, calibrates on N+1, and validates on N+2. Train, calibration, and validation indices are disjoint. The final evaluation trains before 2024, calibrates intervals on 2024, and predicts 2025. Production is then fitted on the approved full population; its in-sample scores are not reported as validation.

## Candidates and selection rule

All candidates receive identical rows, folds, targets, features, and eligibility rules.

- Ridge alpha: `0.01, 0.03, 0.1, 0.3, 1, 2, 5, 10, 30, 100`.
- Elastic Net: alpha `0.01, 0.1, 1` crossed with `l1_ratio` `0.2, 0.8`.
- Histogram gradient boosting: four bounded configurations crossing learning rates `0.05/0.1`, 200/300 maximum iterations, 15/31 leaf nodes, 20–40 minimum samples per leaf, and 0/1/5 L2 regularization.

Primary selection is row-weighted future-period MAE in EUR/m². Ties use P90 APE, absolute signed bias, lower operational complexity, then lexical configuration order. A nonlinear model must lower MAE by at least 3% relative to the best Ridge and win every temporal fold; otherwise Ridge is retained. The 2025 holdout is not accessed by this decision.

### Ridge alpha cross-validation results

Each sequence is in the alpha-grid order above and reports row-weighted MAE.

| Kind/group | Ridge MAE sequence | Retained Ridge alpha, if selected |
|---|---|---:|
| Sale apartment | 662.614, 662.607, 662.583, 662.519, 662.317, 662.057, 661.460, 660.846, 660.247, 661.358 | — |
| Sale commercial | 845.386, 845.379, 845.350, 845.267, 844.986, 844.682, 844.352, 844.702, 847.485, 852.654 | 5 |
| Sale garage | 314.374, 314.354, 314.284, 314.088, 313.440, 312.610, 310.534, 308.072, 303.897, 303.594 | 100 |
| Sale house | 481.521, 481.518, 481.506, 481.463, 481.249, 480.939, 480.282, 479.815, 480.522, 485.223 | 10 |
| Sale land | 53.733, 53.733, 53.733, 53.733, 53.740, 53.751, 53.808, 53.896, 54.200, 54.706 | 0.1 |
| Rent apartment | 2.486, 2.486, 2.486, 2.486, 2.487, 2.487, 2.488, 2.489, 2.491, 2.498 | — |
| Rent commercial | 4.312, 4.312, 4.312, 4.312, 4.312, 4.313, 4.315, 4.318, 4.326, 4.343 | — |
| Rent garage | 2.085, 2.085, 2.085, 2.085, 2.084, 2.084, 2.084, 2.086, 2.092, 2.105 | — |
| Rent house | 2.134, 2.134, 2.133, 2.133, 2.132, 2.132, 2.131, 2.131, 2.139, 2.162 | 5 |
| Rent room | 3.596, 3.596, 3.596, 3.596, 3.595, 3.594, 3.592, 3.591, 3.595, 3.609 | — |

Every candidate configuration, every alpha, every per-fold metric, row-weighted aggregate, and median-fold aggregate is persisted in `analytics.model_runs.selection_metadata` and `metrics.property_groups`.

## Best family cross-validation aggregates

| Kind/group | Best Ridge MAE | Best Elastic Net MAE | Best HGB MAE | Selected |
|---|---:|---:|---:|---|
| Sale apartment | 660.25 | 679.23 | 475.36 | HGB: lr .1, iter 200, leaves 31, min leaf 40, L2 5 |
| Sale commercial | 844.35 | 857.51 | 761.55 | Ridge α=5; HGB was not consistently better across folds |
| Sale garage | 303.59 | 315.07 | 287.70 | Ridge α=100; HGB was not consistently better across folds |
| Sale house | 479.82 | 496.61 | 424.97 | Ridge α=10; HGB was not consistently better across folds |
| Sale land | 53.73 | 54.81 | 53.18 | Ridge α=.1; nonlinear gain was below 3% |
| Rent apartment | 2.49 | 2.55 | 2.37 | HGB: lr .1, iter 200, leaves 31, min leaf 40, L2 5 |
| Rent commercial | 4.31 | 4.38 | 3.72 | HGB: lr .1, iter 200, leaves 31, min leaf 40, L2 5 |
| Rent garage | 2.08 | 2.10 | 1.60 | HGB: lr .05, iter 300, leaves 31, min leaf 30, L2 1 |
| Rent house | 2.13 | 2.18 | 2.02 | Ridge α=5; HGB was not consistently better across folds |
| Rent room | 3.59 | 3.62 | 3.15 | HGB: lr .1, iter 200, leaves 31, min leaf 40, L2 5 |

Elastic Net was rejected for every group because it did not beat the retained alternative. HGB was rejected when its aggregate advantage was negligible or did not repeat in every temporal fold. This prevents replacing Ridge on a single favorable metric.

## Selected-model per-fold results

Compact values are `year: MAE / median APE / P90 APE / empirical coverage`. Full metrics also include P75 APE, signed median percentage error, log RMSE, log R², baseline metrics, row counts, and fit/prediction durations in persisted JSON.

- Sale apartment: 2009 `488.5/20.5%/67.1%/93.2%`; 2010 `407.5/16.3%/61.6%/92.0%`; 2011 `364.7/15.2%/49.3%/91.6%`; 2012 `340.2/14.6%/54.9%/88.2%`; 2013 `394.8/21.0%/89.3%/84.0%`; 2014 `399.0/26.9%/97.3%/87.6%`; 2015 `290.5/17.4%/66.4%/90.5%`; 2016 `295.2/18.1%/49.5%/89.5%`; 2017 `377.1/22.9%/49.5%/86.3%`; 2018 `452.1/25.5%/49.7%/86.9%`; 2019 `450.2/24.8%/48.8%/89.2%`; 2020 `461.5/22.5%/49.1%/89.0%`; 2021 `593.1/27.3%/50.7%/82.6%`; 2022 `808.3/34.3%/54.6%/79.0%`; 2023 `760.8/29.4%/50.1%/89.4%`; 2024 `770.6/26.4%/47.6%/87.5%`.
- Sale commercial: 2009–2024 MAE range `534–6,655`, median APE range `39.1–92.0%`, coverage range `78.7–98.1%`.
- Sale garage: 2009–2024 MAE range `182–504`, median APE range `26.1–42.3%`, coverage range `82.4–93.4%`.
- Sale house: 2009–2024 MAE range `345–763`, median APE range `38.2–56.7%`, coverage range `86.0–92.0%`.
- Sale land: 2012–2024 MAE range `44–85`, median APE range `64.6–88.8%`, coverage range `84.4–92.9%`.
- Rent apartment: 2015–2024 MAE range `1.9–2.9`, median APE range `30.1–37.4%`, coverage range `85.1–89.9%`.
- Rent commercial: 2015–2024 MAE range `3.4–4.0`, median APE range `25.9–34.0%`, coverage range `88.6–90.5%`.
- Rent garage: 2015–2024 MAE range `1.3–2.2`, median APE range `17.4–29.1%`, coverage range `84.3–91.9%`.
- Rent house: 2015–2024 MAE range `1.8–2.5`, median APE range `40.3–46.0%`, coverage range `87.8–93.0%`.
- Rent room: 2015–2024 MAE range `2.6–3.8`, median APE range `26.0–33.6%`, coverage range `84.1–89.1%`.

The complete per-year values are deliberately persisted rather than rounded away in this report.

## Untouched final holdout and conformal intervals

Intervals use the existing finite-sample split-conformal absolute log-residual method with nominal 90% coverage.

| Kind/group | Holdout rows | MAE | Median APE | P90 APE | Signed median error | Coverage | Mean width | Median width |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Sale apartment | 7,319 | €858.14/m² | 28.2% | 47.7% | -25.2% | 84.6% | €2,429 | €2,254 |
| Sale commercial | 1,529 | €905.13/m² | 61.6% | 141.6% | -47.6% | 89.8% | €3,115 | €2,584 |
| Sale garage | 875 | €523.16/m² | 40.6% | 66.4% | -33.6% | 86.4% | €1,435 | €1,410 |
| Sale house | 3,389 | €822.92/m² | 57.3% | 90.2% | -49.2% | 89.6% | €2,426 | €2,085 |
| Sale land | 474 | €134.96/m² | 64.6% | 199.6% | -44.2% | 87.6% | €343 | €229 |
| Rent apartment | 1,336 | €3.84/m² | 36.2% | 60.2% | -33.5% | 90.3% | €12.27 | €11.48 |
| Rent commercial | 1,572 | €4.33/m² | 26.6% | 73.1% | -12.9% | 89.9% | €16.56 | €15.58 |
| Rent garage | 305 | €3.02/m² | 17.7% | 58.4% | -5.3% | 91.1% | €5.89 | €5.61 |
| Rent house | 109 | €5.66/m² | 55.3% | 87.7% | -47.1% | 85.3% | €12.96 | €10.89 |
| Rent room | 108 | €4.38/m² | 25.8% | 65.6% | -11.3% | 91.7% | €23.30 | €17.33 |

Overall sale coverage is 86.7%, materially below nominal; the CLI emits a warning and the system does not claim calibrated 90% empirical coverage. Overall rental coverage is 90.1%. Sparse groups that cannot form valid folds receive an explicit unavailable status and an uncalibrated Ridge production fallback, never a calibrated-coverage claim.

## Grouped robustness

Grouping uses stable cadastral-municipality/building identity where present and municipality fallback otherwise. A deterministic 20% group holdout is separate from temporal selection and has no group overlap.

| Kind/group | Held-out groups | Held-out rows | Log R² |
|---|---:|---:|---:|
| Sale apartment | 4,582 | 29,275 | 0.616 |
| Sale commercial | 2,947 | 4,592 | 0.254 |
| Sale garage | 1,168 | 3,442 | 0.300 |
| Sale house | 10,003 | 11,459 | 0.282 |
| Sale land (municipality fallback) | 43 | 2,884 | 0.014 |
| Rent apartment | 4,398 | 33,576 | 0.330 |
| Rent commercial | 1,673 | 8,461 | 0.294 |
| Rent garage | 251 | 2,064 | 0.237 |
| Rent house | 1,397 | 2,874 | 0.246 |
| Rent room | 822 | 3,144 | 0.271 |

## Persistence and production behavior

Migration `025_model_selection_metadata.sql` adds separate `selection_metadata` and `final_holdout_metrics` JSONB fields. Metadata records model family, all hyperparameters/candidates, temporal definitions, skipped reasons, grouped robustness, fold and aggregate metrics, target transform, feature schema, date ranges, interval method/coverage, seed, row counts, domain exclusions, and measured durations. Existing columns and API reads remain compatible.

Production fits the approved configuration on its full eligible property-group population and persists production predictions. Validation predictions remain transient metrics from models fitted without 2025; they are never replaced by in-sample production scores in validation reporting. Isolation Forest remains separately fitted, deterministic, and untuned by regression accuracy; its anomaly threshold and score persistence are unchanged.

## Runtime and memory

The representative v3 run took 10m25s for sales and 6m56s for rentals. During the search the pipeline worker used about 6.5–6.8 CPU cores and peaked at approximately 878 MiB in its 3 GiB container limit; PostgreSQL used approximately 1.6 GiB. Gradient boosting is materially slower than Ridge, which is reflected in the complexity threshold.

## Verification

Executed successfully:

- targeted model tests, including deterministic repeat selection and holdout-target mutation: 15 tests;
- `make pipeline-lint`;
- `make pipeline-test`: 60 passed;
- clean database application of migrations 001–025 and schema verification;
- rebuilt-container representative `refresh-models --workers 6` on 527,100 selected sale/rental production rows;
- persisted final-holdout evaluation and grouped robustness inspection;
- `make api-test`, including clean-population ETN code 1/2 inclusion and code 3 exclusion;
- `git diff --check`.

## Files changed

- `apps/data-pipeline/real_estate_etl/model_validation.py`
- `apps/data-pipeline/real_estate_etl/model_selection.py`
- `apps/data-pipeline/real_estate_etl/model_features.py`
- `apps/data-pipeline/real_estate_etl/modeling.py`
- `apps/data-pipeline/real_estate_etl/model_reporting.py`
- `apps/data-pipeline/tests/test_modeling.py`
- `apps/data-pipeline/tests/test_model_selection.py`
- `apps/real-estate-api/internal/analytics/statistics_integration_test.go`
- `database/migrations/025_model_selection_metadata.sql`
- `docs/model-optimization-report.md`

## Remaining risks

- Sale intervals under-cover, especially apartments; intervals should be presented with observed coverage and revisited with group-conditional or adaptive conformal methods without using the holdout for tuning.
- Signed holdout errors are negative for every group, showing material underprediction during a rising market. Calendar-year and location features do not fully model regime shifts.
- Commercial, land, and house errors remain high and heavy-tailed. More domain features may help, but feature work needs a fresh predeclared selection cycle and a new untouched future holdout.
- Grouped robustness is not a temporal-geographic double holdout; it measures entity/geographic transfer separately as intended.
- The full offline search remains computationally expensive. Current CLI
  progress reports transaction kind and parallel property-group milestones;
  per-fold detail is available with `MODEL_DETAILED_PROGRESS=true`.
