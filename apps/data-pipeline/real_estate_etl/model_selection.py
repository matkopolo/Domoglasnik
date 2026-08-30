"""Deterministic, leakage-safe valuation model selection."""

from __future__ import annotations

import time
from dataclasses import dataclass
from datetime import date
from typing import Any, Callable

import numpy as np
from sklearn.ensemble import HistGradientBoostingRegressor
from sklearn.linear_model import ElasticNet, Ridge
from sklearn.model_selection import GroupShuffleSplit
from sklearn.pipeline import Pipeline

from .model_features import preprocessor
from .model_validation import (
    NOMINAL_INTERVAL_COVERAGE,
    final_holdout_split,
    interval_half_width,
    interval_metrics,
    regression_metrics,
    temporal_folds,
)

RIDGE_ALPHA_GRID = [0.01, 0.03, 0.1, 0.3, 1.0, 2.0, 5.0, 10.0, 30.0, 100.0]
ELASTIC_NET_GRID = [
    {"alpha": alpha, "l1_ratio": ratio}
    for alpha in (0.01, 0.1, 1.0) for ratio in (0.2, 0.8)
]
HIST_GRADIENT_BOOSTING_GRID = [
    {"learning_rate": 0.05, "max_iter": 200, "max_leaf_nodes": 15,
     "min_samples_leaf": 30, "l2_regularization": 0.0},
    {"learning_rate": 0.05, "max_iter": 300, "max_leaf_nodes": 31,
     "min_samples_leaf": 30, "l2_regularization": 1.0},
    {"learning_rate": 0.1, "max_iter": 200, "max_leaf_nodes": 15,
     "min_samples_leaf": 20, "l2_regularization": 1.0},
    {"learning_rate": 0.1, "max_iter": 200, "max_leaf_nodes": 31,
     "min_samples_leaf": 40, "l2_regularization": 5.0},
]
PRACTICAL_NONLINEAR_MAE_IMPROVEMENT = 0.03


@dataclass(frozen=True)
class SelectionSettings:
    min_training_rows: int = 100
    min_calibration_rows: int = 30
    min_validation_rows: int = 30
    nominal_coverage: float = NOMINAL_INTERVAL_COVERAGE
    random_seed: int = 20260723
    detailed_progress: bool = False


def candidate_configurations() -> list[dict[str, Any]]:
    return (
        [{"family": "ridge", "alpha": value} for value in RIDGE_ALPHA_GRID]
        + [{"family": "elastic_net", **values} for values in ELASTIC_NET_GRID]
        + [{"family": "hist_gradient_boosting", **values}
           for values in HIST_GRADIENT_BOOSTING_GRID]
    )


def build_pipeline(configuration: dict[str, Any], seed: int) -> Pipeline:
    estimator = build_estimator(configuration, seed)
    return Pipeline([
        ("features", preprocessor(list(range(6)), [6, 7], dense=True)),
        ("regression", estimator),
    ])


def build_estimator(configuration: dict[str, Any], seed: int):
    """Build only the regressor for already fold-local preprocessed features."""
    family = configuration["family"]
    if family == "ridge":
        estimator = Ridge(alpha=configuration["alpha"])
    elif family == "elastic_net":
        estimator = ElasticNet(
            alpha=configuration["alpha"], l1_ratio=configuration["l1_ratio"],
            max_iter=5_000, random_state=seed,
        )
    elif family == "hist_gradient_boosting":
        estimator = HistGradientBoostingRegressor(
            **{key: value for key, value in configuration.items() if key != "family"},
            random_state=seed,
        )
    else:
        raise ValueError(f"unknown model family: {family}")
    return estimator


def _timed_fit_predict(model, train_x, train_y, validation_x):
    started = time.monotonic()
    model.fit(train_x, train_y)
    training_seconds = time.monotonic() - started
    started = time.monotonic()
    predicted = model.predict(validation_x)
    return predicted, training_seconds, time.monotonic() - started


def _aggregate(folds: list[dict[str, Any]]) -> dict[str, Any]:
    metric_names = [
        "mae_amount_per_sqm", "median_absolute_percentage_error",
        "p75_absolute_percentage_error", "p90_absolute_percentage_error",
        "signed_median_percentage_error", "rmse_log", "r2_log",
        "empirical_interval_coverage", "mean_interval_width", "median_interval_width",
    ]
    weights = np.asarray([fold["validation_rows"] for fold in folds], dtype=float)
    result: dict[str, Any] = {"fold_count": len(folds), "validation_rows": int(weights.sum())}
    for metric in metric_names:
        values = np.asarray([fold[metric] for fold in folds], dtype=float)
        result[f"weighted_{metric}"] = float(np.average(values, weights=weights))
        result[f"median_fold_{metric}"] = float(np.median(values))
        result[f"std_fold_{metric}"] = float(np.std(values))
    result["training_seconds"] = float(sum(fold["training_seconds"] for fold in folds))
    result["prediction_seconds"] = float(sum(fold["prediction_seconds"] for fold in folds))
    return result


def _selection_key(candidate: dict[str, Any]) -> tuple[Any, ...]:
    aggregate = candidate["aggregate"]
    return (
        aggregate["weighted_mae_amount_per_sqm"],
        aggregate["weighted_p90_absolute_percentage_error"],
        abs(aggregate["weighted_signed_median_percentage_error"]),
        {"ridge": 0, "elastic_net": 1, "hist_gradient_boosting": 2}[candidate["configuration"]["family"]],
        repr(sorted(candidate["configuration"].items())),
    )


def select_configuration(candidates: list[dict[str, Any]]) -> dict[str, Any]:
    ranked = sorted(candidates, key=_selection_key)
    winner = ranked[0]
    ridge = next((item for item in ranked if item["configuration"]["family"] == "ridge"), None)
    if ridge and winner["configuration"]["family"] != "ridge":
        best = winner["aggregate"]["weighted_mae_amount_per_sqm"]
        ridge_mae = ridge["aggregate"]["weighted_mae_amount_per_sqm"]
        improvement = (ridge_mae - best) / ridge_mae
        consistent = all(
            nonlinear["mae_amount_per_sqm"] < linear["mae_amount_per_sqm"]
            for nonlinear, linear in zip(winner["folds"], ridge["folds"])
        )
        if improvement < PRACTICAL_NONLINEAR_MAE_IMPROVEMENT or not consistent:
            winner = ridge
    return winner


def optimize_model(
    features: np.ndarray,
    target: np.ndarray,
    dates: list[date],
    groups: np.ndarray,
    settings: SelectionSettings,
    configurations: list[dict[str, Any]] | None = None,
    progress: Callable[[str], None] | None = None,
) -> dict[str, Any]:
    progress = progress or (lambda _message: None)
    folds, skipped, holdout_year = temporal_folds(
        dates, min_training_rows=settings.min_training_rows,
        min_calibration_rows=settings.min_calibration_rows,
        min_validation_rows=settings.min_validation_rows,
    )
    if holdout_year is None or not folds:
        progress("selection unavailable: insufficient temporal folds")
        return {"status": "insufficient_temporal_folds", "skipped_folds": skipped,
                "final_holdout_year": holdout_year}
    progress(
        f"temporal selection: {len(folds)} valid folds, {len(skipped)} skipped, "
        f"untouched holdout={holdout_year}"
    )
    evaluated = []
    candidates = configurations or candidate_configurations()
    candidate_results: list[list[dict[str, Any]]] = [[] for _ in candidates]
    total_candidate_fits = len(candidates) * len(folds)
    progress_interval = max(1, (total_candidate_fits + 9) // 10)
    completed_candidate_fits = 0
    selection_started = time.monotonic()
    progress(
        f"fit plan: {len(candidates)} candidates × {len(folds)} folds = "
        f"{total_candidate_fits} candidate fits, plus holdout, grouped, and production stages"
    )
    preprocessing_seconds = 0.0
    for fold_number, fold in enumerate(folds, start=1):
        preprocessing_started = time.monotonic()
        fold_preprocessor = preprocessor(list(range(6)), [6, 7], dense=True)
        training_features = fold_preprocessor.fit_transform(features[fold.training])
        calibration_features = fold_preprocessor.transform(features[fold.calibration])
        validation_features = fold_preprocessor.transform(features[fold.validation])
        fold_preprocessing_seconds = time.monotonic() - preprocessing_started
        preprocessing_seconds += fold_preprocessing_seconds
        if settings.detailed_progress:
            progress(
                f"fold {fold_number}/{len(folds)} preprocessing complete "
                f"(train through {fold.train_years[-1]}, "
                f"calibrate {fold.calibration_year}, validate {fold.validation_year})"
            )
        for candidate_number, configuration in enumerate(candidates, start=1):
            if settings.detailed_progress:
                progress(
                    f"fit {completed_candidate_fits + 1}/{total_candidate_fits}: "
                    f"candidate {candidate_number}/{len(candidates)}, "
                    f"fold {fold_number}/{len(folds)} "
                    f"(train through {fold.train_years[-1]}, "
                    f"calibrate {fold.calibration_year}, "
                    f"validate {fold.validation_year})"
                )
            model = build_estimator(configuration, settings.random_seed)
            predicted, training_seconds, prediction_seconds = _timed_fit_predict(
                model, training_features, target[fold.training], validation_features
            )
            calibration_prediction = model.predict(calibration_features)
            half_width = interval_half_width(
                target[fold.calibration], calibration_prediction, settings.nominal_coverage
            )
            candidate_results[candidate_number - 1].append({
                "name": fold.name, "train_years": list(fold.train_years),
                "calibration_year": fold.calibration_year,
                "validation_year": fold.validation_year,
                "training_rows": int(len(fold.training)),
                "calibration_rows": int(len(fold.calibration)),
                "validation_rows": int(len(fold.validation)),
                **regression_metrics(target[fold.validation], predicted),
                **interval_metrics(target[fold.validation], predicted, half_width),
                "baseline": regression_metrics(
                    target[fold.validation],
                    np.full(len(fold.validation), np.median(target[fold.training])),
                ),
                "training_seconds": training_seconds,
                "prediction_seconds": prediction_seconds,
                "shared_preprocessing_seconds": fold_preprocessing_seconds,
            })
            completed_candidate_fits += 1
            elapsed = time.monotonic() - selection_started
            average = elapsed / completed_candidate_fits
            remaining = average * (total_candidate_fits - completed_candidate_fits)
            if (
                settings.detailed_progress
                or completed_candidate_fits % progress_interval == 0
                or completed_candidate_fits == total_candidate_fits
            ):
                progress(
                    f"progress {completed_candidate_fits}/{total_candidate_fits} "
                    f"({100 * completed_candidate_fits / total_candidate_fits:.1f}%): "
                    f"candidate {candidate_number}/{len(candidates)}, "
                    f"fold {fold_number}/{len(folds)}; "
                    f"elapsed={_duration(elapsed)}, ETA={_duration(remaining)}"
                )
    for configuration, results in zip(candidates, candidate_results):
        evaluated.append({"configuration": configuration, "folds": results,
                          "aggregate": _aggregate(results)})
    progress(
        f"shared fold preprocessing: {len(folds)} fits instead of "
        f"{total_candidate_fits}; elapsed={_duration(preprocessing_seconds)}"
    )
    selected = select_configuration(evaluated)
    progress(f"selected: {_configuration_label(selected['configuration'])}")
    training, calibration, holdout = final_holdout_split(
        dates, holdout_year, min_training_rows=settings.min_training_rows,
        min_calibration_rows=settings.min_calibration_rows,
        min_holdout_rows=settings.min_validation_rows,
    )
    final_model = build_pipeline(selected["configuration"], settings.random_seed)
    progress(f"final holdout: fitting pre-{holdout_year} training population")
    holdout_prediction, training_seconds, prediction_seconds = _timed_fit_predict(
        final_model, features[training], target[training], features[holdout]
    )
    calibration_prediction = final_model.predict(features[calibration])
    half_width = interval_half_width(target[calibration], calibration_prediction,
                                     settings.nominal_coverage)
    baseline = np.full(len(holdout), np.median(target[training]))
    final_metrics = {
        **regression_metrics(target[holdout], holdout_prediction),
        **interval_metrics(target[holdout], holdout_prediction, half_width),
        "nominal_interval_coverage": settings.nominal_coverage,
        "calibration_rows": int(len(calibration)),
        "holdout_rows": int(len(holdout)),
        "holdout_year": holdout_year,
        "training_seconds": training_seconds,
        "prediction_seconds": prediction_seconds,
        "baseline": regression_metrics(target[holdout], baseline),
    }
    pre_holdout = np.flatnonzero(np.asarray([value.year for value in dates]) < holdout_year)
    robustness = grouped_robustness(
        features[pre_holdout], target[pre_holdout], groups[pre_holdout],
        selected["configuration"], settings,
    )
    progress(
        "grouped robustness: "
        + (
            f"{robustness['held_out_group_count']} groups / "
            f"{robustness['held_out_row_count']} rows"
            if robustness["status"] == "evaluated"
            else str(robustness["status"])
        )
    )
    return {
        "status": "selected", "candidate_results": evaluated,
        "selected_configuration": selected["configuration"],
        "selection_rule": {
            "primary": "row_weighted_future_period_mae_amount_per_sqm",
            "tie_breakers": ["p90_absolute_percentage_error", "absolute_signed_bias",
                             "prefer_lower_operational_complexity", "configuration_lexical"],
            "nonlinear_minimum_mae_improvement": PRACTICAL_NONLINEAR_MAE_IMPROVEMENT,
            "nonlinear_must_win_every_fold": True,
        },
        "temporal_folds": [fold.definition() for fold in folds],
        "skipped_folds": skipped, "final_holdout_year": holdout_year,
        "final_holdout": final_metrics, "grouped_robustness": robustness,
        "interval_half_width_log": half_width,
        "selection_runtime": {
            "wall_seconds": time.monotonic() - selection_started,
            "shared_preprocessing_seconds": preprocessing_seconds,
            "candidate_estimator_training_seconds": float(sum(
                candidate["aggregate"]["training_seconds"] for candidate in evaluated
            )),
            "candidate_prediction_seconds": float(sum(
                candidate["aggregate"]["prediction_seconds"] for candidate in evaluated
            )),
        },
    }


def _configuration_label(configuration: dict[str, Any]) -> str:
    parameters = ", ".join(
        f"{key}={value}" for key, value in configuration.items() if key != "family"
    )
    return f"{configuration['family']}({parameters})"


def _duration(seconds: float) -> str:
    rounded = max(0, round(seconds))
    minutes, seconds = divmod(rounded, 60)
    hours, minutes = divmod(minutes, 60)
    if hours:
        return f"{hours}h {minutes}m {seconds:02d}s"
    if minutes:
        return f"{minutes}m {seconds:02d}s"
    return f"{seconds}s"


def grouped_robustness(features, target, groups, configuration, settings):
    valid = np.asarray([bool(value) for value in groups])
    distinct = sorted(set(groups[valid]))
    if len(distinct) < 5:
        return {"status": "insufficient_groups", "distinct_groups": len(distinct)}
    local_features, local_target, local_groups = features[valid], target[valid], groups[valid]
    split = GroupShuffleSplit(n_splits=1, test_size=0.2, random_state=settings.random_seed)
    training, validation = next(split.split(local_features, groups=local_groups))
    model = build_pipeline(configuration, settings.random_seed)
    predicted, training_seconds, prediction_seconds = _timed_fit_predict(
        model, local_features[training], local_target[training], local_features[validation]
    )
    return {
        "status": "evaluated", "held_out_group_count": len(set(local_groups[validation])),
        "held_out_row_count": int(len(validation)),
        "groups_disjoint": set(local_groups[training]).isdisjoint(local_groups[validation]),
        **regression_metrics(local_target[validation], predicted),
        "training_seconds": training_seconds, "prediction_seconds": prediction_seconds,
    }
