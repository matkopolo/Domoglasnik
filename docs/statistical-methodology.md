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

Broader same-year benchmarks remain separate. Nearby candidates first require
area within ±15% and construction year within ±10 years and search radii of
500 m, 1 km, 2.5 km, then 5 km. If no radius reaches the minimum sample of
five, the filter widens to ±25% area and ±15 construction years and repeats
the radii. When construction year is absent, the corresponding area-only
profile is used. No unmatched nearby median is silently substituted.

Settlement, city-district, and other small-area comparisons use ±25% area and
±15 construction years (or area only when age is absent). Municipality,
statistical-region, and national medians are intentionally unadjusted, broader
market context. Every benchmark states its profile and numeric tolerances in
the API and interface. The hierarchy therefore ends with the broader
municipality/region context and then the hedonic Ridge estimate. Package
composition and data-quality evidence must still be checked before treating
two agreements as economically equivalent.

## Current-market estimate

The current-market estimate is an additional reference and does not replace
the historical transaction comparisons. For every sale or rental with usable
area and coordinates, the API searches representative transactions from the
current calendar year using progressively larger radii of 1 km, 2.5 km, 5 km,
10 km, and 20 km. It first requires area within ±15% and construction year
within ±10 years. If that strict profile cannot supply 50 observations within
20 km, it widens to ±25% area and ±15 construction years and repeats the
progressive-radius search. If the incomplete current calendar year still
cannot supply 50 observations, the same hierarchy is repeated over the current
and previous calendar years. Search expansion stops once at least 50
observations are available; 20 km remains an absolute ceiling. When the
available market is smaller, the largest eligible cohort of at least five
observations is returned with an explicit warning that the 50-comparable target
was not reached. The response warns whenever the previous year was needed.

Candidates receive a transparent similarity weight:

- 35% relative floor-area similarity;
- 25% construction-year similarity;
- 20% straight-line PostGIS distance similarity;
- 10% contract recency;
- 10% agreement-package component-count similarity.

The central estimate is the weighted median amount per m² multiplied by the
subject area. The displayed lower and upper values are weighted p25 and p75,
not formal appraisal confidence intervals. When fewer than five current-year
candidates belong to the subject municipality, the worker compares matched
transactions in that municipality with the surrounding selected-radius market
over the preceding three calendar years. A factor is applied only with at
least five local and twenty surrounding observations and is bounded to
0.65–1.35. The response exposes the factor and local sample size.

Confidence is moderate only with at least 50 strict matches inside 10 km, low
with at least 20, and very low otherwise. The current year is explicitly
marked incomplete. Property condition, renovation, view, exact floor and
package details can remain unobserved, so the output is not an official
valuation.

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
the same cadastral building part within 30 days, 2% relative area, and 2%
relative amount. They receive a versioned score and `possible_near_duplicate`
flag. They are never automatically deleted because renewals, corrections,
partial interests, and legitimate repeat transactions can look similar.

## Hedonic valuation model

### Model architecture

The valuation component is supervised linear regression with L2
regularization (`sklearn.linear_model.Ridge`) after numeric scaling and
categorical one-hot encoding. “Hedonic” means it estimates value from observed
property, time, and location attributes. Ridge shrinks unstable coefficients
to improve stability while retaining an interpretable model structure.

Isolation Forest is a separate unsupervised tree ensemble. It identifies
unusual combinations for the anomaly-review queue; it does not produce the
valuation and cannot remove a transaction by itself.

The offline Python worker trains separate sale and rental models for each
property group. The target is log(price per m²) for sales and log(monthly rent
per m²) for rentals. Features are:

- log area;
- construction year;
- contract year and month;
- D96/TM centroid easting and northing;
- property group;
- municipality identifier.

Numeric missing values use training-set medians and are standardized.
Categorical features use one-hot encoding, ignore unseen categories, and group
categories occurring fewer than 10 times. The estimator is Ridge regression
with `alpha=2.0`. A fixed seed (`20260723`) makes validation, Isolation Forest,
and bootstrap sampling reproducible.

Ridge is used as an interpretable constant-quality baseline. Its estimate is
not a certified appraisal and can be weak where attributes are missing or
micro-location/property condition is unobserved.

## Honest temporal and spatial validation

For each property group, the latest complete calendar year is the untouched
test set, the preceding complete year is the conformal-calibration set, and
earlier years train the validation model. If year cells are too sparse, a
stable chronological 60/20/20-style split with at least 10 calibration and 10
test rows is used.

A separate `GroupShuffleSplit` holds out 20% of municipalities to measure
spatial generalization. The seed is fixed and held-out municipality count is
recorded. Reported metrics are:

- MAE in original amount-per-m² units;
- median absolute percentage error;
- RMSE of log target;
- R² of log target;
- train-only local-median baseline MAE;
- temporal sample/date ranges and municipality-holdout metrics.

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
- **Local-median baseline improvement:** compares model MAE with a deliberately
  simple prediction learned only from training data. Positive is useful;
  negative means the model did not beat that simple baseline.
- **Spatial R²:** the same generalization check when entire municipalities are
  withheld, revealing whether the model transfers to unseen locations.
- **Interval coverage:** share of test observations inside the nominal 90%
  split-conformal range. Close to 90% is desired; higher coverage can merely
  mean excessively wide intervals, so width and point error also matter.

After training, the worker prints these measures overall and by property
group. It emits warnings when interval coverage is more than three percentage
points below nominal or when temporal/spatial R² is negative. Full metrics,
feature schema, split ranges, random seed, and model version are also persisted
in `analytics.model_runs.metrics` for later auditing.

Example output shape:

```text
Model validation — sale price/m²
  version=... rows=... elapsed=...
  latest-complete-year temporal holdout: n=..., MAE=..., median APE=..., log RMSE=..., log R²=...
  split-conformal interval: nominal=90.0%, observed=...
  property-group diagnostics:
    apartment: test n=..., MAE=..., median APE=..., log R²=..., vs local-median baseline=...; spatial R²=...
  Interpretation: validation error and coverage are diagnostics, not a certified appraisal or a classification accuracy.
```

Only temporal test rows receive honest residual anomaly percentiles. Full-data
models are subsequently trained for production estimates, but in-sample
residuals are not misrepresented as held-out anomaly evidence.

## Split-conformal prediction ranges

Absolute log residuals on the disjoint calibration year determine a 90%
finite-sample conformal half-width:

```text
q = sorted(|actual_log - predicted_log|)[ceil((n + 1) * 0.90) - 1]
range = [exp(predicted_log - q), exp(predicted_log + q)]
```

The implementation records nominal and empirical test coverage and mean log
interval width. The range quantifies model uncertainty under exchangeability;
it is not a guarantee and does not include every legal, physical, or market
risk.

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
are held fixed while only contract year/month is changed. The fitted Ridge
model predicts counterfactual amount per m² for every month. The median
prediction is indexed to the first month (=100).

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
