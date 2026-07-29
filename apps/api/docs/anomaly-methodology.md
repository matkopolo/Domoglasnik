# Anomaly and data-quality methodology

An anomaly is a review signal, not proof of an error or wrongdoing. Source rows
are immutable. Published representative statistics exclude a row only through
a versioned, documented quality policy; anomaly rankings remain separately
queryable.

## Signal layers

1. **Hard validation:** impossible dates, non-positive or implausible area and
   consideration, invalid package composition, invalid rental duration/VAT,
   and coordinates outside Slovenia. These produce named deterministic flags.
2. **Duplicate candidates:** exact source identity is handled by import keys.
   Analytical candidates use transaction date, component identifiers, area,
   amount, and location. They are labelled exact, near, or possible; legitimate
   multi-component packages must not be collapsed. Near candidates compare
   different transaction facts for the same cadastral building part within 30
   days, 2% relative area, and 2% relative amount.
3. **Robust comparable cohorts:** IQR and modified z-scores based on MAD within
   building when possible, otherwise municipality/property type/area band/year,
   with statistical-region fallback and explicit minimum samples.
4. **Hedonic residual:** model log price or rent per m² from property
   attributes, time, and geography. Cross-fitted absolute residuals are
   calibrated within property type and geography; training and scoring periods
   must be recorded.
5. **Isolation Forest:** a secondary multivariate signal for unusual
   combinations. Record feature schema, preprocessing, random seed, model
   version, score, and threshold. It cannot independently exclude a row.
6. **Spatial validation:** PostGIS point-in-polygon checks compare a centroid
   with its claimed municipality and country; boundary tolerance and missing
   coordinates are separate states.
7. **Review queue:** rank the strongest combined signals and retain reviewer
   state, reason, timestamp, and model/rule versions.

## Scoring

The proposed expression
`2 × hardRuleFailures + MADScore + ModelResidualScore + IsolationForestScore`
is useful conceptually but the raw inputs have incompatible scales. Production
scoring therefore maps every signal to `[0,1]`, caps correlated contributions,
and publishes both component scores and the total:

```text
review_score =
    0.35 * hard_rule_score +
    0.20 * robust_cohort_score +
    0.20 * held_out_residual_score +
    0.10 * isolation_forest_score +
    0.10 * duplicate_score +
    0.05 * spatial_score
```

Hard failures may make a row ineligible under an explicit policy, but a high
review score alone does not. Thresholds are calibrated on a labelled review
sample and monitored by property group and geography rather than selected to
force a fixed percentage of anomalies.

## Required flags

At minimum: `invalid_area`, `invalid_amount`, `invalid_date`,
`non_comparable_package`, `extreme_price_per_m2`,
`large_model_residual`, `isolation_forest_anomaly`, `possible_duplicate`,
`coordinate_outside_municipality`, `coordinate_outside_slovenia`, and
`missing_coordinates`.

## Model evaluation

- Use the latest complete calendar year as test, the preceding year for
  split-conformal calibration, and earlier data for training.
- Score model residual anomalies only on held-out rows; rows without an honest
  held-out prediction do not receive a residual anomaly score.
- Report MAE, median absolute percentage error, log RMSE, log R², interval
  coverage, a train-only local-median baseline, and property-group metrics.
- Report a separate deterministic municipality-blocked holdout.
- Check spatial autocorrelation of residuals; remaining structure means the
  model or uncertainty interval is incomplete.
- Prefer interpretable hedonic/GAM baselines before more complex models.
- Never describe an anomaly as fraud based only on transaction data.

## Research basis

- Liu, Ting, and Zhou, *Isolation Forest* (ICDM 2008), as referenced by the
  scikit-learn implementation.
- Robust-statistics practice supports median, IQR, MAD, and resistant
  regression when contaminated observations can distort means and standard
  deviations.
- Hedonic research models property value from attributes and location; large
  residuals are feature-adjusted anomaly signals, while spatial research shows
  residual autocorrelation must be checked.
- OGC Simple Features predicates provide the basis for reproducible
  point-in-polygon spatial validation in PostGIS.
