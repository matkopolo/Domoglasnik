"""Versioned hedonic residual and Isolation Forest anomaly scoring."""

from __future__ import annotations

import json
import math
import os
import time
import uuid
from dataclasses import dataclass
from datetime import date, datetime, timezone

import numpy as np
import psycopg
from sklearn.compose import ColumnTransformer
from sklearn.ensemble import IsolationForest
from sklearn.impute import SimpleImputer
from sklearn.linear_model import Ridge
from sklearn.metrics import mean_absolute_error, mean_squared_error
from sklearn.model_selection import KFold
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OneHotEncoder, StandardScaler

RANDOM_SEED = 20260723
MODEL_FOLDS = 5
ISOLATION_TRAINING_LIMIT = 50_000
MODEL_FEATURES = [
    "log_area",
    "construction_year",
    "contract_year",
    "contract_month",
    "centroid_e",
    "centroid_n",
    "property_group",
    "municipality_id",
]


@dataclass(frozen=True)
class ModelResult:
    transaction_kind: str
    rows: int
    model_version: str
    elapsed_seconds: float
    metrics: dict[str, object]


def _preprocessor(numeric_columns: list[int], categorical_columns: list[int]) -> ColumnTransformer:
    return ColumnTransformer(
        [
            (
                "numeric",
                Pipeline(
                    [
                        ("impute", SimpleImputer(strategy="median", keep_empty_features=True)),
                        ("scale", StandardScaler()),
                    ]
                ),
                numeric_columns,
            ),
            (
                "categorical",
                OneHotEncoder(handle_unknown="ignore", min_frequency=10),
                categorical_columns,
            ),
        ]
    )


def _hedonic_pipeline() -> Pipeline:
    return Pipeline(
        [
            ("features", _preprocessor(list(range(6)), [6, 7])),
            ("regression", Ridge(alpha=2.0)),
        ]
    )


def percentile_scores(values: np.ndarray) -> np.ndarray:
    """Map higher raw anomaly values to deterministic empirical scores in [0,1]."""
    if len(values) == 0:
        return np.array([], dtype=float)
    ordered = np.sort(values)
    return np.searchsorted(ordered, values, side="right") / len(values)


def _query(kind: str) -> str:
    table = "analytics.sale_facts" if kind == "sale" else "analytics.rental_facts"
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
        ORDER BY transaction_id
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
    with connection.cursor() as cursor:
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

    predictions = np.empty(len(rows), dtype=float)
    fitted_predictions = np.empty(len(rows), dtype=float)
    interval_widths = np.empty(len(rows), dtype=float)
    residual_scores = np.empty(len(rows), dtype=float)
    property_groups = features[:, 6]
    group_metrics: dict[str, dict[str, float]] = {}
    final_models: dict[str, Pipeline] = {}
    for group in sorted(set(property_groups)):
        group_indices = np.flatnonzero(property_groups == group)
        group_features = features[group_indices]
        group_target = target[group_indices]
        group_predictions = np.empty(len(group_indices), dtype=float)
        folds = KFold(n_splits=MODEL_FOLDS, shuffle=True, random_state=RANDOM_SEED)
        for training, validation in folds.split(group_features):
            model = _hedonic_pipeline()
            model.fit(group_features[training], group_target[training])
            group_predictions[validation] = model.predict(group_features[validation])
        predictions[group_indices] = group_predictions
        group_residuals = np.abs(group_target - group_predictions)
        residual_scores[group_indices] = percentile_scores(group_residuals)
        group_interval = float(np.quantile(group_residuals, 0.90))
        interval_widths[group_indices] = group_interval
        final_model = _hedonic_pipeline()
        final_model.fit(group_features, group_target)
        final_models[str(group)] = final_model
        fitted_predictions[group_indices] = final_model.predict(group_features)
        group_actual = np.exp(group_target)
        group_estimated = np.exp(group_predictions)
        group_metrics[str(group)] = {
            "rows": int(len(group_indices)),
            "median_absolute_percentage_error": float(
                np.median(np.abs(group_actual - group_estimated) / group_actual)
            ),
            "rmse_log": float(math.sqrt(mean_squared_error(group_target, group_predictions))),
            "empirical_log_half_width": group_interval,
        }

    isolation_features = np.column_stack([target, features])
    isolation_transformer = _preprocessor(list(range(7)), [7, 8])
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
    coverage = float(np.mean((actual >= lower) & (actual <= upper)))
    metrics: dict[str, object] = {
        "mae_amount_per_sqm": float(mean_absolute_error(actual, np.exp(predictions))),
        "median_absolute_percentage_error": float(
            np.median(np.abs(actual - np.exp(predictions)) / actual)
        ),
        "rmse_log": float(math.sqrt(mean_squared_error(target, predictions))),
        "empirical_interval_coverage": coverage,
        "property_groups": group_metrics,
    }
    now = datetime.now(timezone.utc)
    version = f"{kind}-hedonic-ridge-iforest-v1-{now:%Y%m%dT%H%M%SZ}"
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
                            "crossFittingFolds": MODEL_FOLDS,
                            "separateModelPerPropertyGroup": True,
                            "isolationTrainingLimit": ISOLATION_TRAINING_LIMIT,
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
                            float(target[index] - predictions[index]),
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
                DELETE FROM analytics.market_indices
                WHERE transaction_kind=%s AND method='hedonic'
                """,
                (kind,),
            )
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
                    methodology_version
                ) VALUES(%s,'hedonic',%s,NULL,%s,%s,%s,%s,%s,%s)
                """,
                [
                    (
                        kind,
                        group,
                        period,
                        index_value,
                        sample_size,
                        base_period,
                        model_run_id,
                        "constant-quality-ridge-v1",
                    )
                    for group, period, index_value, sample_size, base_period in hedonic_indices
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
) -> list[tuple[str, date, float, int, date]]:
    rows: list[tuple[str, date, float, int, date]] = []
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
        predicted_medians = []
        for period in periods:
            counterfactual = reference.copy()
            counterfactual[:, 2] = float(period.year)
            counterfactual[:, 3] = float(period.month)
            predicted_medians.append(float(np.median(np.exp(model.predict(counterfactual)))))
        base = predicted_medians[0]
        rows.extend(
            (
                str(group),
                period,
                100.0 * predicted_median / base,
                reference_size,
                periods[0],
            )
            for period, predicted_median in zip(periods, predicted_medians)
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
