# Statistical and analytical methodology

## Scope and governing principles

The system analyzes Slovenian GURS ETN purchase data from 2007 onward and
rental data from 2013 onward. It preserves source agreements and components,
then creates one analytical fact per agreement. A house sold together with
land and a garage is one transaction package, not three independent sales.

An anomaly is evidence for review, not proof of an error, fraud, or wrongdoing.
Source rows are never modified by statistical procedures. Missing target
amounts, areas, and coordinates are not imputed. Every published statistic
reports its usable sample and suppression state.

## Analytical populations

### Quality eligibility

SQL fact refreshes apply deterministic rules before statistical comparison:
valid contract date, positive/credible agreement amount and area, coherent
package composition, credible property-group area and amount-per-m² ranges,
and usable rental duration/amount semantics. Each failure is persisted as a
named `quality_reasons` flag.

`is_quality_eligible` means the fact passes those hard rules.
`representative_eligible` additionally means the value was not classified as a
robust cohort outlier. Descriptive endpoints expose raw, eligible, included,
and excluded counts so filtering remains auditable.

### Package price and area

Price per m² is calculated only when the agreement amount and area can be
meaningfully paired. A total package price is not silently assigned to one
apartment component. The fact records `price_derivation`. Components remain
available on transaction detail pages even when a representative unit price
cannot be produced.

### Land

Land-only agreements are classified at transaction grain:

- strict (default): every parcel has verified-buildable ETN code 1 or 2;
- broad: every parcel uses code 1, 2, or 3;
- all: every retained land-only agreement, requested explicitly.

This is an ETN readiness classification, not independent proof of current
zoning or a building permit.

## Descriptive statistics and publication

PostgreSQL calculates counts, arithmetic means, medians, continuous
percentiles, minimum/maximum representative transactions, and monthly,
quarterly, or annual series. Medians and percentiles are the primary measures
because property markets are skewed; means are retained for context.

The versioned publication policy requires at least 30 included observations
for descriptive cells and index periods. Smaller cells remain stored and their
count is returned, but the statistic is marked unpublishable with a suppression
reason. This is statistical reliability suppression, not privacy anonymization.

Year-over-year change compares a period with the same period one year earlier.
No prior compatible observation means no YoY value. Distribution endpoints
return chart bins plus p10, p25, median, p75, and p90 over the selected eligible
population.

## Inflation context

The inflation chart uses the Statistical Office of the Republic of Slovenia
(SURS) SiStat table `H281S`, annual consumer-price index for each month
(month/same month of the previous year). The displayed rate is `index - 100`.
Monthly points are shown directly; quarterly and yearly points are arithmetic
means of the available monthly annual rates in that period. Inflation is a
national macroeconomic reference, not a property-price deflator and not a
geography-specific estimate. Transaction-volume points count all GURS
agreements matching the active sale/rent, property group, geography, area,
date, and frequency filters.

## Transaction-level comparable hierarchy

“Primerjava s trgom” separates direct evidence from aggregate market context.
For a selected sale or rental, direct comparables use the same cadastral
building and property group and rank up to eight quality-eligible transactions:

1. same contract year and area within ±15%;
2. adjacent year and area within ±15%;
3. same year at any observed area.

Within a tier, the smallest relative area difference ranks first. Each result
shows the actual transaction, total amount, amount per m², area difference,
date, and representative/outlier state. One transaction is useful direct
evidence but is not presented as a reliable market median.

Broader same-year benchmarks remain separate. Nearby candidates require area
within ±15% and construction year within ±8 years and search radii of 500 m,
1 km, 2.5 km, then 5 km. The physical tolerances are not widened when the
cohort is sparse. When construction year is absent, the corresponding ±15%
area-only profile is used. No unmatched nearby median is silently substituted.

Settlement, city-district, and other small-area comparisons use ±15% area and
±8 construction years (or area only when age is absent). Municipality,
statistical-region, and national medians are intentionally unadjusted, broader
market context. Every benchmark states its profile and numeric tolerances in
the API and interface. The hierarchy therefore ends with the broader
municipality/region context and then the hedonic Ridge estimate. Package
composition and data-quality evidence must still be checked before treating
two agreements as economically equivalent.

## Current-market estimate

The “Ocena trenutne tržne vrednosti” is an on-request backend calculation from
observed comparable transactions. It is not the trained valuation model and it
does not replace the selected transaction's historical comparisons. The API
returns the two estimates separately as `currentMarketEstimate` and
`valuation`.

### Which transaction years are compared

The estimate answers “what does the latest available market evidence imply
now?”, not “what was this property worth in the year of the selected sale?”
The selected transaction's contract year does not determine the comparison
window and there is no symmetric ±1-year window around it.

The backend uses the server's current calendar year:

1. Search transactions dated from 1 January of the current year through the
   latest imported date before 1 January of the next year.
2. Try to reach 50 comparable transactions using the physical and distance
   hierarchy below.
3. Only when no current-year profile reaches 50, repeat the hierarchy using a
   combined window beginning 1 January of the previous year and ending before
   1 January of the next year.
4. Stop at the first profile/radius containing at least 50 candidates. If none
   reaches 50, retain the largest cohort found across all attempts, but publish
   an estimate only if it contains at least five candidates.

For example, during 2026 the first window contains only imported 2026
transactions. The fallback contains all available 2025 transactions plus the
incomplete 2026 observations. A property originally sold in 2017 is still
estimated from these latest windows; 2016–2018 are used only by its separate
historical transaction-context comparisons, not by the current-market card.
Because GURS deliveries and the current year can be incomplete, the response
always warns that the current calendar year is unfinished and adds another
warning when the previous year was required.

### Comparable search order

The candidate must have the same transaction kind and property group as the
subject, be representative-eligible, have positive area and amount per m²,
have coordinates, and not be the subject transaction itself. Sale-land
comparisons use the strict verified-buildable land population. The query is
capped at the nearest 250 eligible candidates for each attempted cohort.
Candidates with the same deterministic contractual package signature are
searched first. For sales the signature is the ordered multiset of ETN building
part and parcel type codes; for rentals it is the ordered multiset of rented
space type codes. This distinguishes an apartment with parking from an
apartment with a different auxiliary component even when both packages contain
two rows. A mixed-composition cohort is allowed only when fewer than five
exact-signature matches exist, and the fallback is returned as an explicit
warning.

The higher transaction ID in every persisted near-duplicate candidate pair is
excluded from current-market and same-building comparable populations, matching
the valuation-model training rule. The source transaction remains stored and
auditable.

The search uses progressively larger radii of 1 km, 2.5 km, 5 km, 10 km, and
20 km:

1. area within ±15% and construction year within ±8 years;
2. do not widen those physical tolerances merely to reach the target sample;
3. if construction year is unavailable, use the corresponding ±15%
   area-only profile;
4. if the current year remains too thin, repeat the search over the combined
   current-plus-previous-year window.

The radius is therefore adaptive rather than always 20 km. A result with 50
strict candidates inside 2.5 km stops there. A sparse market may expand to
20 km and the previous year, but remains physically strict and may therefore
return fewer than 50 candidates. The response records the actual maximum radius,
area tolerance, construction-year tolerance, candidate count, and reference
year used by the calculation.

Same-building transactions are displayed separately as direct comparables.
They receive no special hard-coded priority in the current-market calculation,
although they can naturally receive high weight when they satisfy the filters
because their distance and physical similarity are strong.

### Weighting and calculation

Candidates receive a transparent similarity weight:

- 35% relative floor-area similarity;
- 25% construction-year similarity;
- 20% straight-line PostGIS distance similarity;
- 10% contract recency over the latest two years;
- 10% agreement-package component-signature similarity.

In implementation terms, a perfect area match contributes 35 points and falls
linearly to zero at a 15% difference. A perfect construction-year match
contributes 25 points and falls to zero at an 8-year difference; missing age
information receives a neutral 12.5 points. Distance contributes 20 points at
the same location and falls linearly to zero at 20 km. Recency contributes up
to 10 points and falls to zero after approximately two years. An exact
component-type signature in the transaction package contributes 10 points. Every
candidate receives a minimum total weight of one.

The central estimate is the weighted median amount per m² multiplied by the
subject area. The displayed lower and upper values are weighted p25 and p75,
not formal appraisal confidence intervals. For a sale, the amount is sale
price per m²; for a rental, it is monthly rent per m². Multiplication by the
subject's recorded area produces the displayed total sale value or monthly
rent range. Package transactions can contain additional property components,
so the result remains an estimate of the recorded package rather than a
component-level certified valuation.

### Sparse-local-market adjustment

When fewer than five selected current-market candidates belong to the subject
municipality, the API estimates a historical local-location factor. It compares
the median of physically matched transactions in the subject municipality with
the median of the surrounding selected-radius market over the three completed
calendar years before the current year. For a 2026 request, that adjustment
window is 2023–2025 and excludes incomplete 2026.

The factor is applied only with at least twenty local and fifty total matched
historical observations. The raw municipality-to-surrounding-market ratio is
then shrunk toward `1.0` with weight `local_n / (local_n + 30)` and the applied
factor is bounded to `0.80–1.20`. Otherwise the applied factor remains `1.0`.
The response exposes the raw factor, applied factor, whether adjustment was
applied, and the local sample size. It also returns both the unadjusted
comparable range and the adjusted range, so the effect remains inspectable.

### Confidence and returned evidence

Confidence is moderate only with at least 50 strict matches inside 10 km, low
with at least 20, and very low otherwise. The current year is explicitly
marked incomplete. Property condition, renovation, view, exact floor and
package details can remain unobserved, so the output is not an official
valuation.

The response exposes:

- weighted p25, median, and p75 in amount per m² and total amount;
- comparable count and actual maximum radius;
- area and construction-year tolerances;
- location-adjustment factor and supporting local sample size;
- current reference year, confidence label, warnings, and methodology version
  `adaptive-current-market-v4`.

## Robust cohort outliers

Sales compare price per m² using the most specific cohort with enough data:

1. cadastral building;
2. municipality + property group + area band + contract year;
3. statistical region + property group + area band + contract year.

Sales require at least 5 building observations, 15 municipality/year
observations, or 30 statistical-region/year observations. Rentals use the
municipality (minimum 15) then regional (minimum 30) cohorts for monthly rent
per m²; they have no building-level fallback.

For observations `x`, PostgreSQL calculates Q1, median, Q3, IQR = Q3 − Q1,
and, for sales, MAD = median(|x − median|). Rentals approximate MAD as
IQR/1.349. A row is an outlier if either:

```text
x < Q1 - 1.5 * max(IQR, 0.20 * |median|)
x > Q3 + 1.5 * max(IQR, 0.20 * |median|)
```

or its stabilized modified z-score exceeds 3.5:

```text
0.6745 * |x - median| / max(MAD, 0.10 * |median|) > 3.5
```

The relative floors prevent zero/tiny IQR or MAD from making nearly identical
small cohorts hypersensitive. Persisted evaluations include cohort level,
size, median, bounds, score, status, reason, and rule version. Representative
averages exclude detected cohort outliers; audit endpoints still expose them.

## Duplicate detection

Import uniqueness removes repeated source rows by archive checksum and source
line identity. That is distinct from analytical duplicate detection.

Exact candidate signals compare transaction date, component identity, area,
amount, and location. Near candidates compare different transaction facts for
the same cadastral building part within 7 days, 2% relative area, and 2%
relative amount. They receive a versioned score and `possible_near_duplicate`
flag. They are never automatically deleted because renewals, corrections,
partial interests, and legitimate repeat transactions can look similar.
Within each candidate group the lowest transaction ID remains the canonical
record. Only higher transaction IDs receive the possible-duplicate flag and
are excluded from valuation-model and comparable populations; the canonical
record is not labelled as duplicated merely because a later matching row exists.

## Hedonic valuation model

### Publication and data freshness

New or changed GURS archives do not immediately replace published analytical
results. The pipeline rebuilds sale and rental facts, robust outlier
classifications, shared quality analytics, indices, and the valuation models
as one pending analytical release. The prior release remains queryable during
calculation and validation. One transaction publishes the replacement only
after every required stage succeeds. This operational consistency mechanism
does not alter any eligibility, outlier, validation, or selection definition.

The imported source cutoff and published analytical cutoff are deliberately
separate. The UI can therefore report that newer GURS evidence is being
processed while stating exactly through which date the active verified
analytics apply.

### Model architecture

The valuation component is supervised hedonic regression: it estimates the
conditional sale price or rent from observed property, time, and location
attributes. Model selection compares three estimator families:

- Ridge regression with ten L2 regularization strengths;
- Elastic Net with six alpha/L1-ratio combinations;
- histogram gradient boosting with four bounded configurations.

Ridge and Elastic Net are regularized linear models. Histogram gradient
boosting can learn nonlinear relationships and interactions. A nonlinear model
is retained only when its row-weighted future-period MAE improves on the best
Ridge model by at least 3% **and** it beats Ridge in every valid temporal fold.
Otherwise the more stable and operationally simpler Ridge model is retained.
The final untouched holdout is never used to choose the family or parameters.

The evaluated grids are deliberately bounded:

- Ridge `alpha`: `0.01, 0.03, 0.1, 0.3, 1, 2, 5, 10, 30, 100`;
- Elastic Net `alpha`: `0.01, 0.1, 1`, each with `l1_ratio` `0.2` and `0.8`;
- histogram gradient boosting: four combinations of learning rate `0.05/0.1`,
  200/300 iterations, 15/31 leaf nodes, 20–40 minimum rows per leaf, and
  L2 regularization `0/1/5`.

Primary selection uses row-weighted future-period MAE. Deterministic
tie-breakers are P90 absolute percentage error, absolute signed bias, lower
operational complexity, and then the lexical configuration order.

### End-to-end training execution and fit count

One explicit `refresh-models` run processes sales and rentals separately. For
each populated property group within that transaction kind, it performs the
following sequence:

1. Construct leakage-safe expanding temporal folds and reserve the latest
   complete calendar year as an untouched final holdout.
2. Evaluate 20 candidate configurations:
   - 10 Ridge configurations;
   - 6 Elastic Net configurations;
   - 4 histogram-gradient-boosting configurations.
3. Fit every candidate independently on every valid temporal fold. Each fit
   owns preprocessing learned only from that fold's training rows. The
   following year calibrates its conformal interval, and the next year measures
   future-period validation error.
4. Aggregate fold metrics and select the configuration without consulting the
   final holdout.
5. Fit the selected configuration on the pre-holdout training population,
   calibrate on the preceding complete year, and evaluate exactly once on the
   untouched final holdout.
6. Fit the selected configuration once more on a deterministic building-first
   grouped split to measure transfer to unseen buildings or fallback
   municipalities.
7. Fit the approved production configuration on every eligible row in that
   property group, including the latest holdout and incomplete current year.
8. Generate production predictions, conformal ranges, residual signals, and
   the property group's constant-quality hedonic index.

If a property group has `F` valid temporal folds, the approximate supervised
fit count is:

```text
20 × F candidate/fold fits
+ 1 final-holdout fit
+ 1 grouped-robustness fit
+ 1 full-population production fit
= 20F + 3 supervised fits
```

For an apartment population with 16 valid temporal folds, that is:

```text
20 × 16 = 320 candidate/fold fits
+ 1 final-holdout fit
+ 1 grouped-robustness fit
+ 1 production fit
= approximately 323 supervised fits
```

The actual total changes by property group because available years and minimum
row thresholds determine the number of valid folds. Skipped folds do not
produce a fit and retain an explicit reason. A sparse group that cannot form
valid temporal selection uses one uncalibrated production fallback instead of
claiming cross-validated selection.

After every property group for one transaction kind is complete, the worker
trains one separate Isolation Forest across the combined eligible sale or
rental population. Its deterministic training sample is capped at 50,000 rows;
the resulting unusualness scores are converted to percentiles separately
within each property group. Isolation Forest is not one of the 20 supervised
candidates and is not selected using valuation error.

Finally, the worker persists one versioned model run, all selected production
predictions, anomaly signals, interval metadata, candidate/fold metrics,
holdout metrics, grouped robustness, and hedonic-index vintages. Validation
predictions and full-population production predictions remain conceptually
separate.

Property groups are computationally independent, so the worker runs up to
`--workers` groups concurrently for one transaction kind. Sales and rentals
remain sequential to bound peak memory, and all database writes remain on the
main connection after the parallel fits finish. The Docker runtime restricts
BLAS, OpenMP, MKL, and NumExpr to one native thread per fit, preventing nested
thread pools from multiplying the configured concurrency. This parallelism
changes wall-clock time only; it does not change rows, folds, seed, candidate
order, selection rules, or persisted predictions.

Within a temporal fold, all 20 candidates use the same training rows and the
same preprocessing definition. The worker therefore fits median imputation,
scaling, categorical-frequency thresholds, and one-hot encoding **once inside
that fold**, transforms that fold's calibration and validation rows, and then
reuses those matrices for the 20 candidate estimators. It never reuses a
transformer between folds. This removes 19 duplicate preprocessing fits per
fold without exposing future rows or changing a candidate's inputs, fitted
estimator, predictions, or selection metrics. The transformed matrices are
released as the worker advances to the next fold to keep memory bounded.
Persisted candidate `training_seconds` measures estimator fitting only;
fold-local shared preprocessing and total optimization wall time are recorded
separately so the same preprocessing duration is not counted twenty times.

Progress logs expose candidate-fit totals and completion rather than implying
one opaque training operation. They report transaction kind, property group,
candidate, completed/total candidate fits at approximately 10% milestones,
elapsed time, and a rolling ETA. Per-fold years are available through the
`MODEL_DETAILED_PROGRESS` diagnostic setting but are suppressed by default to
keep concurrent group output readable. Holdout, grouped robustness, production
fitting, Isolation Forest, index generation, and persistence are named stages
because they are outside the `20 × F` candidate-fit counter.

Isolation Forest is a separate unsupervised tree ensemble. It identifies
unusual combinations for the anomaly-review queue; it does not produce the
valuation and cannot remove a transaction by itself.

### What constitutes one model

The worker does **not** train one model per statistical region or one model per
municipality. It trains a national model for every available combination of:

```text
transaction kind × property group
```

Sales and rentals are always separate. Apartments, houses, commercial
property, garages, land, and rooms are separated where that kind contains the
group. For example, the representative run produced five sale model
populations and five rental model populations. A Slovenian municipality is an
input category, not a separate model, and statistical region is not currently
a model feature. Coordinates allow the national model to learn broader spatial
patterns while municipality one-hot encoding captures municipality-level
differences supported by training data.

This design pools evidence across Slovenia instead of fitting unstable models
for small municipalities. Its limitation is that an unseen municipality has
no learned category coefficient; the encoder safely ignores it and the model
must rely on its other features. Building and municipality holdouts measure
this transfer risk separately.

The target is `log(price per m²)` for sales and `log(monthly rent per m²)` for
rentals. Features are:

- log area;
- construction year;
- contract year and month;
- D96/TM centroid easting and northing;
- property group;
- municipality identifier.

Numeric missing values use training-set medians and are standardized.
Categorical features use one-hot encoding, ignore unseen categories, and group
categories occurring fewer than 10 times. All imputation, scaling, and
encoding are fitted inside each training fold; validation and future rows do
not influence preprocessing. A fixed seed (`20260723`) makes candidate
selection, grouped validation, Isolation Forest, and bootstrap sampling
reproducible.

The selected estimator is a constant-quality statistical estimate, not a
certified appraisal. It can be weak where attributes are missing or where
condition, renovation quality, exact view, legal state, or micro-location is
unobserved.

### Which rows are eligible

The training query starts from quality-eligible analytical fact rows with a
positive amount-per-m² target, positive area, a contract date, and usable
model features. Sales begin in 2007 and rentals in 2013. The current calendar
year is excluded from validation because it is incomplete, but it is included
when the approved production model is finally fitted.

Raw ETN rows remain stored. Model exclusion does not delete source data.
Persisted near-duplicate pairs cannot inflate folds: the higher transaction ID
from each candidate pair is excluded. Default sale-land training uses the
strict land view and therefore includes only verified-buildable ETN land codes
1 and 2; code 3, mixed, unknown, and non-buildable land are excluded.

Model populations are not fixed-size samples. Every eligible row for the given
transaction kind and property group is used. The representative 2026 run used:

| Population | Production rows |
|---|---:|
| Sale apartments | 153,664 |
| Sale commercial property | 25,552 |
| Sale garages | 16,925 |
| Sale houses | 62,144 |
| Sale verified-buildable land | 14,048 |
| Rental apartments | 168,326 |
| Rental commercial property | 45,675 |
| Rental garages | 9,738 |
| Rental houses | 14,212 |
| Rental rooms | 16,816 |

These counts describe the recorded run, not permanent quotas. They change when
GURS data, quality rules, or duplicate evidence changes. The authoritative
counts and exclusions for each run are persisted in
`analytics.model_runs.metrics` and `selection_metadata`.

## Honest temporal and spatial validation

### Rolling model-selection folds

Validation is performed independently for each transaction-kind/property-group
population. Complete calendar years before the latest complete year form
expanding-window folds:

```text
Fold k:
  train       = every eligible year through N
  calibrate   = year N+1
  validate    = year N+2

Final evaluation:
  train       = every year before the calibration year
  calibrate   = the year immediately before the holdout
  holdout     = the latest complete calendar year
```

With an August 2026 run, 2026 is incomplete, 2025 is the untouched holdout,
and 2024 is its conformal-calibration year. Candidate selection uses only folds
ending no later than 2024. Changing 2025 target values cannot change the
selected family or hyperparameters. The selected configuration is evaluated
exactly once on 2025.

The default minimum sizes are 100 training rows, 30 calibration rows, and 30
validation/holdout rows. They can be configured with
`MODEL_MIN_TRAINING_ROWS`, `MODEL_MIN_CALIBRATION_ROWS`, and
`MODEL_MIN_VALIDATION_ROWS`. A fold below a minimum is skipped with its exact
reason and row counts persisted. There is no shuffled K-fold or 60/20/20
fallback. If a property group cannot form valid folds, production uses an
explicit uncalibrated `Ridge(alpha=2.0)` sparse-group fallback and does not
claim validated accuracy or calibrated 90% coverage.

In the representative run, sales validated future years 2009–2024 (land
2012–2024 after three sparse early folds were skipped), and rentals validated
2015–2024. Exact per-fold training, calibration, and validation row counts are
stored because an expanding training window means there is no single fixed
“number used for learning.”

After selection and the one-time holdout evaluation, the approved
configuration is fitted again on **all** eligible rows for its property group,
including the holdout and incomplete current year. This is the production
model used to persist estimates. Its in-sample predictions are operational
outputs and are never reported as cross-validation or holdout accuracy. The
90% production interval width retains the conformal calibration result obtained
without the final holdout.

### Grouped geographic/entity robustness

Temporal validation remains the primary selection evidence. A separate,
deterministic `GroupShuffleSplit` withholds 20% of entity groups from all
pre-holdout rows to ask whether the selected configuration transfers to unseen
entities. The group key is:

1. stable `cadastral_municipality_code:building_number` when available;
2. municipality identifier when no building identity exists.

Therefore the same known building cannot occur in both grouped training and
validation. Land commonly uses municipality fallback because it has no
building identity. This is not “training a municipality model” and it is not a
combined future-year-plus-unseen-geography test; it is an additional robustness
diagnostic. The representative run held out 4,582 sale-apartment groups
(29,275 rows), while verified-buildable land held out 43 municipality groups
(2,884 rows). Full group counts for every property group are in
`model-optimization-report.md` and persisted model metadata.

Reported metrics include:

- MAE in original amount-per-m² units;
- median absolute percentage error;
- 75th and 90th percentile absolute percentage error;
- signed median percentage error (directional bias);
- RMSE of log target;
- R² of log target;
- empirical interval coverage and mean/median interval width;
- training-population median baseline metrics;
- per-fold row counts and fit/prediction durations;
- grouped building/municipality holdout metrics.

There is no honest single “model accuracy” percentage for this regression
problem. The logged measures answer different questions:

- **MAE amount/m²:** average absolute miss in EUR/m² for sales or monthly
  EUR/m² for rents; lower is better.
- **Median APE:** the typical relative absolute error; 0% is perfect. Unlike
  mean percentage error, one very large miss does not dominate it.
- **Log RMSE:** penalizes large multiplicative errors on the modeled log
  target; lower is better.
- **Log R²:** improvement over predicting the test-set mean on log scale; 1 is
  perfect, 0 is no better, and a negative value is worse than that baseline.
- **Training-median baseline improvement:** compares model MAE with a
  deliberately simple prediction learned only from the fold's training
  population. Positive is useful; negative means the model did not beat that
  simple baseline.
- **Grouped R²:** the same generalization check when entire buildings (or
  fallback municipalities) are withheld, revealing whether the model transfers
  to unseen entities and locations.
- **Interval coverage:** share of test observations inside the nominal 90%
  split-conformal range. Close to 90% is desired; higher coverage can merely
  mean excessively wide intervals, so width and point error also matter.

After training, the worker prints the main measures overall and by property
group and warns when overall interval coverage is more than three percentage
points below nominal. Full candidate, fold, holdout, grouped-robustness and
interval metrics, feature schema, split ranges, random seed, durations, and
model version are persisted for auditing; the console is intentionally a
summary rather than the complete evidence record.

Example output shape:

```text
Model validation — sale price/m²
  version=... rows=... elapsed=...
  latest-complete-year temporal holdout: n=..., MAE=..., median APE=..., log RMSE=..., log R²=...
  split-conformal interval: nominal=90.0%, observed=...
  property-group diagnostics:
    apartment: test n=..., MAE=..., median APE=..., log R²=..., vs training-median baseline=...; grouped R²=...
  Interpretation: validation error and coverage are diagnostics, not a certified appraisal or a classification accuracy.
```

Production-model residual percentiles are assigned only to rows from the final
holdout year for anomaly ranking. Because the production estimator is later
fitted on the full population, these persisted residual scores are operational
in-sample signals, not holdout-validation evidence. The honest validation
errors remain the separately persisted fold and final-holdout metrics.

## Split-conformal prediction ranges

Absolute log residuals on the disjoint calibration year determine a 90%
finite-sample conformal half-width:

```text
q = sorted(|actual_log - predicted_log|)[ceil((n + 1) * 0.90) - 1]
range = [exp(predicted_log - q), exp(predicted_log + q)]
```

The implementation records nominal and empirical holdout coverage plus mean
and median interval width after converting bounds back to EUR/m². The range
quantifies model uncertainty under exchangeability; it is not a guarantee and
does not include every legal, physical, or market risk.

## Isolation Forest

Isolation Forest supplies a secondary multivariate unusualness signal using
log amount per m² plus the hedonic feature set. Preprocessing uses numeric
median imputation/scaling and categorical one-hot encoding. Training is a
deterministic random sample capped at 50,000 rows, with 150 trees,
`contamination="auto"`, and bounded worker parallelism.

Negative `score_samples` values are converted to within-property-group
percentile ranks. Scores at or above the 99th percentile receive
`isolation_forest_anomaly`. Isolation Forest cannot independently exclude a
transaction from published statistics.

## Spatial validation

PostGIS assigns current country, statistical region, municipality, city
district, local community, and settlement with `ST_Covers`. A centroid outside
the national polygon receives a spatial signal; missing coordinates are a
separate state. Spatial assignment is based on current RPE geometry and does
not establish historical legal boundaries or parcel accuracy.

## Combined anomaly review score

All component scores are normalized to `[0,1]` and persisted separately:

```text
review_score = min(1,
  0.35 * hard_rule_score +
  0.20 * robust_cohort_score +
  0.20 * held_out_residual_score +
  0.10 * isolation_forest_score +
  0.10 * duplicate_score +
  0.05 * spatial_score)
```

Named flags explain why a record is ranked. Human review state, reason,
reviewer, timestamp, and rule/model versions persist across refreshes. A high
score creates a review priority; it does not assert fraud and does not mutate
the GURS row.

## Market indices

### Matched-pair median

Consecutive eligible sales of the same cadastral building part, separated by
at least 180 days, produce annualized log returns. Each yearly group uses the
median return and compounds it from a base of 100. The 2.5th and 97.5th
percentiles of pair returns form empirical dispersion bounds.

This is accurately named `matched_pair_median`; it is not a Case-Shiller-style
weighted repeat-sales regression. Periods below 30 pairs are retained but not
published.

### Constant-quality hedonic index

For each property group, at most 5,000 deterministic reference observations
are held fixed while only contract year/month is changed. The selected and
production-fitted estimator for that property group predicts counterfactual
amount per m² for every month. The median prediction is indexed to the first
month (=100).

One hundred deterministic bootstrap resamples of the fixed reference basket
produce 2.5th/97.5th percentile index bounds. Publication requires at least 30
actual transactions in the effective month even though the reference basket
is larger. Every refresh appends an index-calculation/model vintage; the
current view selects the newest vintage without deleting history.

## Missingness and monitoring

Missingness profiles count total/present rows for key fields by transaction
kind, property group, year, and national or municipality scope. Refresh runs,
model versions, feature schemas, training ranges, random seeds, validation
metrics, publication versions, and suppression reasons make outputs
reproducible and diagnosable.

## Known limitations

- ETN records transactions, not complete property condition, renovation,
  financing, ownership share semantics, or motivation.
- Package totals may prevent a defensible component-level price per m².
- Current centroids/polygons do not reconstruct historical cadastral facts.
- Thin cohorts and unseen local conditions can produce broad or unreliable
  predictions; subgroup and spatial R² may be low or negative.
- Conformal validity depends on calibration/test comparability.
- Pair-return bounds are dispersion, not confidence intervals from a weighted
  repeat-sales estimator.
- No anomaly score alone establishes an erroneous or suspicious legal act.

## Implementation map

- SQL populations/rules: migrations `010`–`016` and `020`–`022`.
- Pair index and vintages: migrations `019` and `021`.
- Feature pipeline: `apps/data-pipeline/real_estate_etl/model_features.py`.
- splits, metrics, and conformal math:
  `apps/data-pipeline/real_estate_etl/model_validation.py`.
- fitting, scoring, persistence, and hedonic index:
  `apps/data-pipeline/real_estate_etl/modeling.py`.
- refresh dependency order: `apps/data-pipeline/real_estate_etl/operations.py`.
- API query/publication behavior: `apps/real-estate-api/internal/analytics`.
