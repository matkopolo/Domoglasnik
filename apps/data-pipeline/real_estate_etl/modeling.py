"""Versioned hedonic residual and Isolation Forest anomaly scoring."""

from __future__ import annotations

import json
import os
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import date, datetime, timezone

import numpy as np
import psycopg
from sklearn.ensemble import IsolationForest
from sklearn.pipeline import Pipeline

from .model_features import MODEL_FEATURES, preprocessor
from .model_selection import SelectionSettings, build_pipeline, optimize_model
from .model_validation import NOMINAL_INTERVAL_COVERAGE, percentile_scores

RANDOM_SEED = 20260723
ISOLATION_TRAINING_LIMIT = 50_000


def _format_duration(seconds: float) -> str:
    rounded = max(0, round(seconds))
    minutes, seconds = divmod(rounded, 60)
    hours, minutes = divmod(minutes, 60)
    if hours:
        return f"{hours}h {minutes}m {seconds:02d}s"
    if minutes:
        return f"{minutes}m {seconds:02d}s"
    return f"{seconds}s"


def _environment_flag(name: str) -> bool:
    return os.getenv(name, "").strip().lower() in {"1", "true", "yes", "on"}


@dataclass(frozen=True)
class ModelResult:
    transaction_kind: str
    rows: int
    model_version: str
    elapsed_seconds: float
    metrics: dict[str, object]


@dataclass(frozen=True)
class PropertyGroupResult:
    group: str
    indices: np.ndarray
    predictions: np.ndarray
    interval_widths: np.ndarray
    residual_scores: np.ndarray
    model: Pipeline
    metrics: dict[str, object]


def _train_property_group(
    kind: str,
    group: str,
    group_number: int,
    group_count: int,
    features: np.ndarray,
    target: np.ndarray,
    contract_dates: list[date],
    building_identities: np.ndarray,
    property_groups: np.ndarray,
    settings: SelectionSettings,
) -> PropertyGroupResult:
    group_indices = np.flatnonzero(property_groups == group)
    group_features = features[group_indices]
    group_target = target[group_indices]
    group_dates = [contract_dates[index] for index in group_indices]
    prefix = f"[{kind} model · {group} · group {group_number}/{group_count}]"
    print(f"{prefix} population={len(group_indices):,} rows", flush=True)
    robustness_groups = building_identities[group_indices].copy()
    missing_identity = np.asarray([not value for value in robustness_groups])
    robustness_groups[missing_identity] = np.asarray(
        [f"municipality:{value}" for value in group_features[missing_identity, 7]],
        dtype=object,
    )
    selection = optimize_model(
        group_features,
        group_target,
        group_dates,
        robustness_groups,
        settings,
        progress=lambda message: print(f"{prefix} {message}", flush=True),
    )
    predictions = np.empty(len(group_indices), dtype=float)
    widths = np.empty(len(group_indices), dtype=float)
    residuals = np.zeros(len(group_indices), dtype=float)
    if selection["status"] != "selected":
        fallback = build_pipeline({"family": "ridge", "alpha": 2.0}, RANDOM_SEED)
        print(f"{prefix} fitting sparse-group production fallback", flush=True)
        production_started = time.monotonic()
        fallback.fit(group_features, group_target)
        production_training_seconds = time.monotonic() - production_started
        production_started = time.monotonic()
        predictions[:] = fallback.predict(group_features)
        production_prediction_seconds = time.monotonic() - production_started
        widths[:] = float(np.quantile(
            np.abs(group_target - predictions), NOMINAL_INTERVAL_COVERAGE
        ))
        metrics = {
            **selection,
            "population_rows": int(len(group_indices)),
            "production_fallback": {"family": "ridge", "alpha": 2.0},
            "production": {
                "training_rows": int(len(group_indices)),
                "training_seconds": production_training_seconds,
                "prediction_seconds": production_prediction_seconds,
            },
            "interval_status": "uncalibrated_sparse_group_fallback",
        }
        return PropertyGroupResult(
            group, group_indices, predictions, widths, residuals, fallback, metrics
        )

    widths[:] = float(selection["interval_half_width_log"])
    final_model = build_pipeline(selection["selected_configuration"], RANDOM_SEED)
    print(f"{prefix} fitting selected production model", flush=True)
    production_started = time.monotonic()
    final_model.fit(group_features, group_target)
    production_training_seconds = time.monotonic() - production_started
    production_started = time.monotonic()
    predictions[:] = final_model.predict(group_features)
    production_prediction_seconds = time.monotonic() - production_started
    holdout_year = selection["final_holdout_year"]
    holdout_indices = np.flatnonzero(
        np.asarray([value.year for value in group_dates]) == holdout_year
    )
    holdout_prediction = final_model.predict(group_features[holdout_indices])
    residuals[holdout_indices] = percentile_scores(
        np.abs(group_target[holdout_indices] - holdout_prediction)
    )
    metrics = {
        "population_rows": int(len(group_indices)),
        **selection,
        "production": {
            "training_rows": int(len(group_indices)),
            "training_seconds": production_training_seconds,
            "prediction_seconds": production_prediction_seconds,
        },
    }
    return PropertyGroupResult(
        group, group_indices, predictions, widths, residuals, final_model, metrics
    )


def _train_property_groups(
    kind: str,
    groups: list[str],
    workers: int,
    features: np.ndarray,
    target: np.ndarray,
    contract_dates: list[date],
    building_identities: np.ndarray,
    property_groups: np.ndarray,
    settings: SelectionSettings,
) -> list[PropertyGroupResult]:
    group_workers = min(workers, len(groups))
    print(
        f"[{kind} model] property groups={len(groups)}; parallel workers={group_workers}; "
        "database persistence remains single-threaded",
        flush=True,
    )
    started = time.monotonic()
    results: list[PropertyGroupResult] = []
    with ThreadPoolExecutor(
        max_workers=group_workers, thread_name_prefix=f"{kind}-model"
    ) as executor:
        futures = [
            executor.submit(
                _train_property_group,
                kind,
                group,
                number,
                len(groups),
                features,
                target,
                contract_dates,
                building_identities,
                property_groups,
                settings,
            )
            for number, group in enumerate(groups, start=1)
        ]
        for completed, future in enumerate(as_completed(futures), start=1):
            results.append(future.result())
            elapsed = time.monotonic() - started
            remaining = elapsed / completed * (len(groups) - completed)
            print(
                f"[{kind} model] group completion {completed}/{len(groups)} "
                f"({100 * completed / len(groups):.1f}%); "
                f"elapsed={_format_duration(elapsed)}; ETA={_format_duration(remaining)}",
                flush=True,
            )
    return results


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
          property_group,coalesce(municipality_id,0)::text,contract_date,
          CASE WHEN cadastral_municipality_code IS NOT NULL
                     AND building_number IS NOT NULL
               THEN concat(cadastral_municipality_code,':',building_number)
               ELSE NULL END building_identity
        FROM {table}
        WHERE is_quality_eligible AND {target}>0 AND area>0
          AND contract_date IS NOT NULL
          AND contract_date>=DATE '{minimum_date}'
          AND NOT EXISTS (
            SELECT 1 FROM analytics.near_duplicate_candidates duplicate
            WHERE duplicate.transaction_kind='{kind}'
              AND duplicate.candidate_transaction_id={table}.transaction_id
          )
        ORDER BY contract_date,transaction_id
    """


MODEL_KIND_DESCRIPTIONS = {
    "sale": (
        "learn sale price per m² separately for each property group from "
        "quality-eligible, de-duplicated transactions (strict ETN land rules); "
        "select and calibrate with time-ordered validation before production fitting"
    ),
    "rent": (
        "learn monthly rent per m² separately for each supported property group "
        "from quality-eligible, de-duplicated agreements; select and calibrate "
        "with time-ordered validation before production fitting"
    ),
}

MODEL_OUTPUT_TABLES = (
    "analytics.model_runs",
    "analytics.valuation_predictions",
    "analytics.anomaly_signals",
    "analytics.index_calculation_runs",
    "analytics.market_indices",
    "analytics.model_publications",
    "analytics.pipeline_state",
)


def refresh_models(
    connection: psycopg.Connection,
    workers: int | None = None,
) -> list[ModelResult]:
    workers = workers or max(1, min(6, os.cpu_count() or 1))
    results = []
    all_started = time.monotonic()
    print(
        f"[models] training plan: sale then rent; up to {workers} parallel "
        "property-group workers per transaction kind",
        flush=True,
    )
    for kind_number, kind in enumerate(("sale", "rent"), start=1):
        print(f"[{kind} model] {MODEL_KIND_DESCRIPTIONS[kind]}", flush=True)
        print(
            f"[{kind} model] database tables populated/updated: "
            + ", ".join(MODEL_OUTPUT_TABLES),
            flush=True,
        )
        print(f"[models] transaction kind {kind_number}/2 starting: {kind}", flush=True)
        results.append(_refresh_kind(connection, kind, workers))
        elapsed = time.monotonic() - all_started
        remaining = elapsed / kind_number * (2 - kind_number)
        print(
            f"[models] transaction kind completion {kind_number}/2 "
            f"({50 * kind_number:.1f}%); elapsed={_format_duration(elapsed)}; "
            f"ETA={_format_duration(remaining)}",
            flush=True,
        )
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute(
            """
            UPDATE analytics.pipeline_state
            SET model_bootstrap_completed_at=now(),
                model_bootstrap_version=%s
            WHERE singleton
            """,
            (",".join(result.model_version for result in results),),
        )
        cursor.executemany(
            """
            INSERT INTO analytics.model_publications(
                transaction_kind,model_run_id,activated_at
            )
            SELECT %s,id,clock_timestamp() FROM analytics.model_runs
            WHERE transaction_kind=%s AND model_version=%s
            ON CONFLICT(transaction_kind) DO UPDATE SET
                model_run_id=excluded.model_run_id,
                activated_at=excluded.activated_at
            """,
            [
                (result.transaction_kind, result.transaction_kind, result.model_version)
                for result in results
            ],
        )
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
    building_identities = np.asarray([row[11] for row in rows], dtype=object)

    fitted_predictions = np.empty(len(rows), dtype=float)
    interval_widths = np.empty(len(rows), dtype=float)
    residual_scores = np.zeros(len(rows), dtype=float)
    property_groups = features[:, 6]
    group_metrics: dict[str, dict[str, object]] = {}
    final_models: dict[str, Pipeline] = {}
    selection_settings = SelectionSettings(
        min_training_rows=int(os.getenv("MODEL_MIN_TRAINING_ROWS", "100")),
        min_calibration_rows=int(os.getenv("MODEL_MIN_CALIBRATION_ROWS", "30")),
        min_validation_rows=int(os.getenv("MODEL_MIN_VALIDATION_ROWS", "30")),
        random_seed=RANDOM_SEED,
        detailed_progress=_environment_flag("MODEL_DETAILED_PROGRESS"),
    )
    groups = sorted(str(value) for value in set(property_groups))
    group_workers = min(workers, len(groups))
    for result in _train_property_groups(
        kind, groups, workers, features, target, contract_dates,
        building_identities, property_groups, selection_settings,
    ):
        fitted_predictions[result.indices] = result.predictions
        interval_widths[result.indices] = result.interval_widths
        residual_scores[result.indices] = result.residual_scores
        final_models[result.group] = result.model
        group_metrics[result.group] = result.metrics

    isolation_features = np.column_stack([target, features])
    print(f"[{kind} model] fitting separate Isolation Forest", flush=True)
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
    selected_groups = [value for value in group_metrics.values() if value.get("status") == "selected"]
    holdout_rows = sum(value["final_holdout"]["holdout_rows"] for value in selected_groups)
    def weighted_holdout(metric: str) -> float | None:
        if not holdout_rows:
            return None
        return float(sum(
            value["final_holdout"][metric] * value["final_holdout"]["holdout_rows"]
            for value in selected_groups
        ) / holdout_rows)
    metrics: dict[str, object] = {
        "rows": holdout_rows,
        "mae_amount_per_sqm": weighted_holdout("mae_amount_per_sqm"),
        "median_absolute_percentage_error": weighted_holdout("median_absolute_percentage_error"),
        "rmse_log": weighted_holdout("rmse_log"),
        "r2_log": weighted_holdout("r2_log"),
        "empirical_interval_coverage": weighted_holdout("empirical_interval_coverage"),
        "nominal_interval_coverage": NOMINAL_INTERVAL_COVERAGE,
        "interval_method": "split_conformal_absolute_log_residual",
        "validation_method": "rolling_expanding_temporal_cv_plus_untouched_latest_complete_year",
        "training_parallelism": {
            "requested_workers": workers,
            "property_group_workers": group_workers,
            "database_persistence_workers": 1,
            "native_threads_per_fit": 1,
        },
        "property_groups": group_metrics,
        "excluded_rows_and_reasons": {
            "quality_ineligible": "excluded_by_is_quality_eligible",
            "missing_or_nonpositive_target_or_area": "excluded_before_training",
            "current_calendar_year": "excluded_from_validation_but_included_in_production_fit",
            "near_duplicates": "higher_transaction_id_in_each_persisted_candidate_pair_excluded",
            "strict_land": "default sale view permits only verified-buildable ETN land codes 1 and 2",
        },
    }
    now = datetime.now(timezone.utc)
    version = f"{kind}-selected-hedonic-iforest-v3-{now:%Y%m%dT%H%M%SZ}"
    model_run_id = uuid.uuid4()
    print(f"[{kind} model] persisting model run and production predictions", flush=True)

    with connection.transaction():  # noqa: SIM117 - transaction and cursor have distinct cleanup semantics
        with connection.cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO analytics.model_runs(
                    id,transaction_kind,model_version,model_type,feature_schema,
                    training_rows,training_from,training_to,metrics,random_seed,
                    selection_metadata,final_holdout_metrics
                ) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                """,
                (
                    model_run_id,
                    kind,
                    version,
                    "selected_log_hedonic_plus_isolation_forest",
                    json.dumps(
                        {
                            "hedonic": MODEL_FEATURES,
                            "targetTransformation": "natural_log_amount_per_sqm",
                            "isolationForest": ["log_amount_per_sqm", *MODEL_FEATURES],
                            "separateModelPerPropertyGroup": True,
                            "isolationTrainingLimit": ISOLATION_TRAINING_LIMIT,
                            "validation": {
                                "temporal": "expanding_window_cv_untouched_latest_complete_year",
                                "grouped": "building_identity_then_municipality_group_holdout",
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
                    json.dumps(
                        {
                            group: {
                                key: value
                                for key, value in group_result.items()
                                if key != "final_holdout"
                            }
                            for group, group_result in group_metrics.items()
                        }
                    ),
                    json.dumps(
                        {
                            group: group_result.get("final_holdout")
                            for group, group_result in group_metrics.items()
                            if group_result.get("status") == "selected"
                        }
                    ),
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
                INSERT INTO analytics.valuation_predictions(
                    transaction_kind,transaction_id,model_run_id,
                    predicted_amount_per_sqm,lower_amount_per_sqm,
                    upper_amount_per_sqm,actual_amount_per_sqm,log_residual
                )
                SELECT %s,transaction_id,%s,predicted,lower_bound,upper_bound,
                       actual,log_residual
                FROM model_scores
                ON CONFLICT(transaction_kind,transaction_id,model_run_id) DO UPDATE SET
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
    for group in sorted(models):
        model = models[group]
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
