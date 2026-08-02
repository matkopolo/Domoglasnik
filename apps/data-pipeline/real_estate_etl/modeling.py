"""Versioned hedonic residual and Isolation Forest anomaly scoring."""

from __future__ import annotations

import json
import os
import time
import uuid
from dataclasses import dataclass
from datetime import date, datetime, timezone

import numpy as np
import psycopg
from sklearn.ensemble import IsolationForest
from sklearn.metrics import mean_absolute_error
from sklearn.model_selection import GroupShuffleSplit
from sklearn.pipeline import Pipeline

from .model_features import MODEL_FEATURES, hedonic_pipeline, preprocessor
from .model_validation import (
    NOMINAL_INTERVAL_COVERAGE,
    interval_half_width,
    percentile_scores,
    regression_metrics,
    temporal_split_indices,
)

RANDOM_SEED = 20260723
ISOLATION_TRAINING_LIMIT = 50_000


@dataclass(frozen=True)
class ModelResult:
    transaction_kind: str
    rows: int
    model_version: str
    elapsed_seconds: float
    metrics: dict[str, object]


def _query(kind: str) -> str:
    table = (
        "analytics.sale_facts_strict_land"
        if kind == "sale"
        else "analytics.rental_facts"
    )
    target = "price_per_sqm" if kind == "sale" else "monthly_rent_per_sqm"
    minimum_date = "2007-01-01" if kind == "sale" else "2013-01-01"
    return f"""
        SELECT transaction_id,{target}::float8,ln(area::float8),
          construction_year::float8,extract(year FROM contract_date)::float8,
          extract(month FROM contract_date)::float8,
          ST_X(location)::float8,ST_Y(location)::float8,
          property_group,coalesce(municipality_id,0)::text,
          contract_date
        FROM {table}
        WHERE is_quality_eligible AND {target}>0 AND area>0
          AND contract_date IS NOT NULL
          AND contract_date>=DATE '{minimum_date}'
        ORDER BY contract_date,transaction_id
    """


def refresh_models(
    connection: psycopg.Connection,
    workers: int | None = None,
) -> list[ModelResult]:
    workers = workers or max(1, min(6, os.cpu_count() or 1))
    results = []
    for kind in ("sale", "rent"):
        results.append(_refresh_kind(connection, kind, workers))
    return results


def _refresh_kind(
    connection: psycopg.Connection,
    kind: str,
    workers: int,
) -> ModelResult:
    started = time.monotonic()
    # Keep the training read in its own completed transaction. Without this
    # boundary, psycopg starts an implicit outer transaction for the SELECT;
    # the later persistence block becomes a savepoint and can be rolled back
    # when the system refresh releases its advisory lock.
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute(_query(kind))
        rows = cursor.fetchall()
    if len(rows) < 100:
        raise RuntimeError(f"{kind} model requires at least 100 eligible rows")

    transaction_ids = np.asarray([row[0] for row in rows], dtype=np.int64)
    target = np.log(np.asarray([row[1] for row in rows], dtype=float))
    features = np.asarray(
        [
            [
                row[2],
                row[3],
                row[4],
                row[5],
                row[6],
                row[7],
                row[8],
                row[9],
            ]
            for row in rows
        ],
        dtype=object,
    )
    contract_dates = [row[10] for row in rows]

    fitted_predictions = np.empty(len(rows), dtype=float)
    interval_widths = np.empty(len(rows), dtype=float)
    residual_scores = np.zeros(len(rows), dtype=float)
    property_groups = features[:, 6]
    group_metrics: dict[str, dict[str, object]] = {}
    final_models: dict[str, Pipeline] = {}
    temporal_actual: list[np.ndarray] = []
    temporal_estimated: list[np.ndarray] = []
    temporal_covered: list[np.ndarray] = []
    for group in sorted(set(property_groups)):
        group_indices = np.flatnonzero(property_groups == group)
        group_features = features[group_indices]
        group_target = target[group_indices]
        group_dates = [contract_dates[index] for index in group_indices]
        training, calibration, test = temporal_split_indices(group_dates)
        validation_model = hedonic_pipeline()
        validation_model.fit(group_features[training], group_target[training])
        calibration_predictions = validation_model.predict(group_features[calibration])
        group_interval = interval_half_width(
            group_target[calibration], calibration_predictions
        )
        test_predictions = validation_model.predict(group_features[test])
        test_covered = (
            np.abs(group_target[test] - test_predictions) <= group_interval
        )
        temporal_actual.append(group_target[test])
        temporal_estimated.append(test_predictions)
        temporal_covered.append(test_covered)
        residual_scores[group_indices[test]] = percentile_scores(
            np.abs(group_target[test] - test_predictions)
        )
        interval_widths[group_indices] = group_interval

        spatial_metrics: dict[str, object]
        municipalities = group_features[:, 7]
        if len(set(municipalities)) >= 2:
            spatial_split = GroupShuffleSplit(
                n_splits=1, test_size=0.20, random_state=RANDOM_SEED
            )
            spatial_training, spatial_test = next(
                spatial_split.split(group_features, groups=municipalities)
            )
            spatial_model = hedonic_pipeline()
            spatial_model.fit(
                group_features[spatial_training], group_target[spatial_training]
            )
            spatial_metrics = regression_metrics(
                group_target[spatial_test],
                spatial_model.predict(group_features[spatial_test]),
            )
            spatial_metrics["held_out_municipalities"] = int(
                len(set(municipalities[spatial_test]))
            )
        else:
            spatial_metrics = {
                "rows": 0,
                "status": "insufficient_distinct_municipalities",
            }

        final_model = hedonic_pipeline()
        final_model.fit(group_features, group_target)
        final_models[str(group)] = final_model
        fitted_predictions[group_indices] = final_model.predict(group_features)
        local_median = float(np.median(group_target[training]))
        group_metrics[str(group)] = {
            "population_rows": int(len(group_indices)),
            "temporal": {
                **regression_metrics(group_target[test], test_predictions),
                "train_from": min(group_dates[index] for index in training).isoformat(),
                "train_to": max(group_dates[index] for index in training).isoformat(),
                "calibration_from": min(
                    group_dates[index] for index in calibration
                ).isoformat(),
                "calibration_to": max(
                    group_dates[index] for index in calibration
                ).isoformat(),
                "test_from": min(group_dates[index] for index in test).isoformat(),
                "test_to": max(group_dates[index] for index in test).isoformat(),
                "empirical_interval_coverage": float(np.mean(test_covered)),
                "mean_interval_width_log": 2 * group_interval,
                "local_median_baseline_mae": float(
                    mean_absolute_error(
                        np.exp(group_target[test]),
                        np.full(len(test), np.exp(local_median)),
                    )
                ),
            },
            "spatial": spatial_metrics,
        }

    isolation_features = np.column_stack([target, features])
    isolation_transformer = preprocessor(list(range(7)), [7, 8])
    transformed = isolation_transformer.fit_transform(isolation_features)
    generator = np.random.default_rng(RANDOM_SEED)
    training_size = min(ISOLATION_TRAINING_LIMIT, len(rows))
    training_indices = generator.choice(len(rows), size=training_size, replace=False)
    isolation = IsolationForest(
        n_estimators=150,
        max_samples="auto",
        contamination="auto",
        random_state=RANDOM_SEED,
        n_jobs=workers,
    )
    isolation.fit(transformed[training_indices])
    isolation_raw = -isolation.score_samples(transformed)
    isolation_scores = np.empty(len(rows), dtype=float)
    for group in sorted(set(property_groups)):
        group_indices = np.flatnonzero(property_groups == group)
        isolation_scores[group_indices] = percentile_scores(isolation_raw[group_indices])

    predicted = np.exp(fitted_predictions)
    lower = np.exp(fitted_predictions - interval_widths)
    upper = np.exp(fitted_predictions + interval_widths)
    actual = np.exp(target)
    validation_actual = np.concatenate(temporal_actual)
    validation_estimated = np.concatenate(temporal_estimated)
    coverage = float(np.mean(np.concatenate(temporal_covered)))
    metrics: dict[str, object] = {
        **regression_metrics(validation_actual, validation_estimated),
        "empirical_interval_coverage": coverage,
        "nominal_interval_coverage": NOMINAL_INTERVAL_COVERAGE,
        "interval_method": "split_conformal_absolute_log_residual",
        "validation_method": "latest_complete_year_with_prior_year_calibration",
        "property_groups": group_metrics,
    }
    now = datetime.now(timezone.utc)
    version = f"{kind}-hedonic-ridge-iforest-v2-{now:%Y%m%dT%H%M%SZ}"
    model_run_id = uuid.uuid4()

    with connection.transaction():  # noqa: SIM117 - transaction and cursor have distinct cleanup semantics
        with connection.cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO analytics.model_runs(
                    id,transaction_kind,model_version,model_type,feature_schema,
                    training_rows,training_from,training_to,metrics,random_seed
                ) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                """,
                (
                    model_run_id,
                    kind,
                    version,
                    "ridge_log_hedonic_plus_isolation_forest",
                    json.dumps(
                        {
                            "hedonic": MODEL_FEATURES,
                            "isolationForest": ["log_amount_per_sqm", *MODEL_FEATURES],
                            "separateModelPerPropertyGroup": True,
                            "isolationTrainingLimit": ISOLATION_TRAINING_LIMIT,
                            "validation": {
                                "temporal": "latest_complete_year_with_prior_year_calibration",
                                "spatial": "municipality_group_shuffle_holdout",
                                "interval": "split_conformal_absolute_log_residual",
                                "nominalCoverage": NOMINAL_INTERVAL_COVERAGE,
                            },
                            "landPopulation": (
                                "strict_etn_codes_1_2" if kind == "sale" else None
                            ),
                        }
                    ),
                    len(rows),
                    min(contract_dates),
                    max(contract_dates),
                    json.dumps(metrics),
                    RANDOM_SEED,
                ),
            )
            cursor.execute("DROP TABLE IF EXISTS model_scores")
            cursor.execute(
                """
                CREATE TEMP TABLE model_scores(
                    transaction_id bigint,
                    predicted numeric,
                    lower_bound numeric,
                    upper_bound numeric,
                    actual numeric,
                    log_residual numeric,
                    residual_score numeric,
                    isolation_score numeric
                ) ON COMMIT DROP
                """
            )
            with cursor.copy(
                """
                COPY model_scores(
                    transaction_id,predicted,lower_bound,upper_bound,actual,
                    log_residual,residual_score,isolation_score
                ) FROM STDIN
                """
            ) as copy:
                for index, transaction_id in enumerate(transaction_ids):
                    copy.write_row(
                        (
                            int(transaction_id),
                            float(predicted[index]),
                            float(lower[index]),
                            float(upper[index]),
                            float(actual[index]),
                            float(target[index] - fitted_predictions[index]),
                            float(residual_scores[index]),
                            float(isolation_scores[index]),
                        )
                    )
            cursor.execute(
                """
                DELETE FROM analytics.valuation_predictions prediction
                WHERE prediction.transaction_kind=%s
                  AND NOT EXISTS (
                    SELECT 1 FROM model_scores score
                    WHERE score.transaction_id=prediction.transaction_id
                  )
                """,
                (kind,),
            )
            cursor.execute(
                """
                INSERT INTO analytics.valuation_predictions(
                    transaction_kind,transaction_id,model_run_id,
                    predicted_amount_per_sqm,lower_amount_per_sqm,
                    upper_amount_per_sqm,actual_amount_per_sqm,log_residual
                )
                SELECT %s,transaction_id,%s,predicted,lower_bound,upper_bound,
                       actual,log_residual
                FROM model_scores
                ON CONFLICT(transaction_kind,transaction_id) DO UPDATE SET
                    model_run_id=excluded.model_run_id,
                    predicted_amount_per_sqm=excluded.predicted_amount_per_sqm,
                    lower_amount_per_sqm=excluded.lower_amount_per_sqm,
                    upper_amount_per_sqm=excluded.upper_amount_per_sqm,
                    actual_amount_per_sqm=excluded.actual_amount_per_sqm,
                    log_residual=excluded.log_residual,
                    scored_at=now()
                """,
                (kind, model_run_id),
            )
            cursor.execute(
                """
                UPDATE analytics.anomaly_signals signal
                SET model_residual_score=score.residual_score,
                    isolation_forest_score=score.isolation_score,
                    model_version=%s,
                    flags=array_cat(
                      array_remove(array_remove(signal.flags,
                        'large_model_residual'),'isolation_forest_anomaly'),
                      array_remove(ARRAY[
                        CASE WHEN score.residual_score>=0.99
                             THEN 'large_model_residual' END,
                        CASE WHEN score.isolation_score>=0.99
                             THEN 'isolation_forest_anomaly' END
                      ],NULL)
                    ),
                    review_score=least(1.0,
                      0.35*signal.hard_rule_score+
                      0.20*signal.robust_cohort_score+
                      0.20*score.residual_score+
                      0.10*score.isolation_score+
                      0.10*signal.duplicate_score+
                      0.05*signal.spatial_score
                    )
                FROM model_scores score
                WHERE signal.transaction_kind=%s
                  AND signal.transaction_id=score.transaction_id
                """,
                (version, kind),
            )
            cursor.execute(
                """
                UPDATE analytics.anomaly_signals signal
                SET model_residual_score=NULL,isolation_forest_score=NULL,
                    model_version=NULL,
                    flags=array_remove(array_remove(signal.flags,
                      'large_model_residual'),'isolation_forest_anomaly'),
                    review_score=least(1.0,
                      0.35*signal.hard_rule_score+
                      0.20*signal.robust_cohort_score+
                      0.10*signal.duplicate_score+
                      0.05*signal.spatial_score
                    )
                WHERE signal.transaction_kind=%s
                  AND NOT EXISTS (
                    SELECT 1 FROM model_scores score
                    WHERE score.transaction_id=signal.transaction_id
                  )
                """,
                (kind,),
            )
            cursor.execute(
                """
                INSERT INTO analytics.index_calculation_runs(
                    transaction_kind,method,methodology_version,data_from,data_to,
                    parameters
                ) VALUES(%s,'hedonic','constant-quality-ridge-v2',%s,%s,%s)
                RETURNING id
                """,
                (
                    kind,
                    min(contract_dates),
                    max(contract_dates),
                    json.dumps(
                        {
                            "minimumPeriodObservations": 30,
                            "bootstrapReplicates": 100,
                            "nominalCoverage": 0.95,
                            "landPopulation": (
                                "strict_etn_codes_1_2" if kind == "sale" else None
                            ),
                        }
                    ),
                ),
            )
            index_run_id = cursor.fetchone()[0]
            hedonic_indices = _hedonic_indices(
                property_groups,
                features,
                contract_dates,
                final_models,
                generator,
            )
            cursor.executemany(
                """
                INSERT INTO analytics.market_indices(
                    transaction_kind,method,property_group,geography_id,period,
                    index_value,sample_size,base_period,model_run_id,
                    methodology_version,calculation_run_id,observed_sample_size,
                    reference_sample_size,lower_index_value,upper_index_value,
                    nominal_coverage,uncertainty_method,is_publishable,
                    suppression_reason
                ) VALUES(
                    %s,'hedonic',%s,NULL,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,
                    %s,.95,'deterministic_reference_bootstrap',%s,%s
                )
                """,
                [
                    (
                        kind,
                        group,
                        period,
                        index_value,
                        observed_size,
                        base_period,
                        model_run_id,
                        "constant-quality-ridge-v2",
                        index_run_id,
                        observed_size,
                        reference_size,
                        lower_value,
                        upper_value,
                        observed_size >= 30,
                        None
                        if observed_size >= 30
                        else "below_minimum_period_observations_30",
                    )
                    for (
                        group,
                        period,
                        index_value,
                        observed_size,
                        reference_size,
                        base_period,
                        lower_value,
                        upper_value,
                    ) in hedonic_indices
                ],
            )
    return ModelResult(
        transaction_kind=kind,
        rows=len(rows),
        model_version=version,
        elapsed_seconds=time.monotonic() - started,
        metrics=metrics,
    )


def _hedonic_indices(
    property_groups: np.ndarray,
    features: np.ndarray,
    contract_dates: list[date],
    models: dict[str, Pipeline],
    generator: np.random.Generator,
) -> list[tuple[str, date, float, int, int, date, float, float]]:
    rows: list[tuple[str, date, float, int, int, date, float, float]] = []
    date_values = np.asarray(contract_dates, dtype=object)
    for group, model in models.items():
        indices = np.flatnonzero(property_groups == group)
        reference_size = min(5_000, len(indices))
        reference_indices = generator.choice(indices, size=reference_size, replace=False)
        reference = features[reference_indices].copy()
        group_dates = date_values[indices]
        first = min(group_dates)
        last = max(group_dates)
        periods = list(_month_range(first, last))
        predicted_medians: list[float] = []
        bootstrap_medians: list[np.ndarray] = []
        bootstrap_indices = generator.integers(
            0, reference_size, size=(100, reference_size)
        )
        for period in periods:
            counterfactual = reference.copy()
            counterfactual[:, 2] = float(period.year)
            counterfactual[:, 3] = float(period.month)
            values = np.exp(model.predict(counterfactual))
            predicted_medians.append(float(np.median(values)))
            bootstrap_medians.append(np.median(values[bootstrap_indices], axis=1))
        base = predicted_medians[0]
        bootstrap_base = bootstrap_medians[0]
        for period, predicted_median, bootstraps in zip(
            periods, predicted_medians, bootstrap_medians
        ):
            bootstrap_index = 100.0 * bootstraps / bootstrap_base
            observed_size = sum(
                value.year == period.year and value.month == period.month
                for value in group_dates
            )
            rows.append(
                (
                    str(group),
                    period,
                    100.0 * predicted_median / base,
                    observed_size,
                    reference_size,
                    periods[0],
                    float(np.quantile(bootstrap_index, 0.025)),
                    float(np.quantile(bootstrap_index, 0.975)),
                )
            )
    return rows


def _month_range(first: date, last: date):
    current = date(first.year, first.month, 1)
    end = date(last.year, last.month, 1)
    while current <= end:
        yield current
        current = (
            date(current.year + 1, 1, 1)
            if current.month == 12
            else date(current.year, current.month + 1, 1)
        )
