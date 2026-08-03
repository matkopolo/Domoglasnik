"""Leakage-safe temporal folds, conformal intervals, and valuation metrics."""

from __future__ import annotations

import math
from dataclasses import asdict, dataclass
from datetime import date

import numpy as np
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score

NOMINAL_INTERVAL_COVERAGE = 0.90


@dataclass(frozen=True)
class TemporalFold:
    name: str
    train_years: tuple[int, ...]
    calibration_year: int
    validation_year: int
    training: np.ndarray
    calibration: np.ndarray
    validation: np.ndarray

    def definition(self) -> dict[str, object]:
        result = asdict(self)
        for key in ("training", "calibration", "validation"):
            result[f"{key}_rows"] = int(len(result.pop(key)))
        return result


def temporal_folds(
    contract_dates: list[date],
    *,
    as_of: date | None = None,
    min_training_rows: int = 100,
    min_calibration_rows: int = 30,
    min_validation_rows: int = 30,
) -> tuple[list[TemporalFold], list[dict[str, object]], int | None]:
    """Build expanding folds, reserving the latest complete year as holdout."""
    as_of = as_of or date.today()
    years = np.asarray([value.year for value in contract_dates], dtype=int)
    complete_years = sorted(int(value) for value in set(years[years < as_of.year]))
    if not complete_years:
        return [], [{"status": "skipped", "reason": "no_complete_calendar_years"}], None
    holdout_year = complete_years[-1]
    selection_years = complete_years[:-1]
    folds: list[TemporalFold] = []
    skipped: list[dict[str, object]] = []
    for position in range(2, len(selection_years)):
        calibration_year = selection_years[position - 1]
        validation_year = selection_years[position]
        train_years = tuple(selection_years[: position - 1])
        training = np.flatnonzero(np.isin(years, train_years))
        calibration = np.flatnonzero(years == calibration_year)
        validation = np.flatnonzero(years == validation_year)
        counts = {
            "training": int(len(training)),
            "calibration": int(len(calibration)),
            "validation": int(len(validation)),
        }
        reason = next(
            (
                f"insufficient_{name}_rows:{counts[name]}<{minimum}"
                for name, minimum in (
                    ("training", min_training_rows),
                    ("calibration", min_calibration_rows),
                    ("validation", min_validation_rows),
                )
                if counts[name] < minimum
            ),
            None,
        )
        name = f"through_{train_years[-1]}_cal_{calibration_year}_val_{validation_year}"
        if reason:
            skipped.append({"name": name, "status": "skipped", "reason": reason, **counts})
            continue
        folds.append(
            TemporalFold(
                name, train_years, calibration_year, validation_year,
                training, calibration, validation,
            )
        )
    if not folds and not skipped:
        skipped.append({"status": "skipped", "reason": "fewer_than_four_complete_years"})
    return folds, skipped, holdout_year


def final_holdout_split(
    contract_dates: list[date],
    holdout_year: int,
    *,
    min_training_rows: int,
    min_calibration_rows: int,
    min_holdout_rows: int,
) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    years = np.asarray([value.year for value in contract_dates], dtype=int)
    prior = sorted(int(value) for value in set(years[years < holdout_year]))
    if not prior:
        raise ValueError("final holdout has no earlier calibration year")
    calibration_year = prior[-1]
    parts = (
        np.flatnonzero(years < calibration_year),
        np.flatnonzero(years == calibration_year),
        np.flatnonzero(years == holdout_year),
    )
    minima = (min_training_rows, min_calibration_rows, min_holdout_rows)
    labels = ("training", "calibration", "holdout")
    for label, values, minimum in zip(labels, parts, minima):
        if len(values) < minimum:
            raise ValueError(f"insufficient_{label}_rows:{len(values)}<{minimum}")
    return parts


def percentile_scores(values: np.ndarray) -> np.ndarray:
    if len(values) == 0:
        return np.array([], dtype=float)
    ordered = np.sort(values)
    return np.searchsorted(ordered, values, side="right") / len(values)


def interval_half_width(actual_log: np.ndarray, predicted_log: np.ndarray,
                        coverage: float = NOMINAL_INTERVAL_COVERAGE) -> float:
    if len(actual_log) == 0 or len(actual_log) != len(predicted_log):
        raise ValueError("calibration arrays must be non-empty and equal length")
    rank = min(len(actual_log) - 1, math.ceil((len(actual_log) + 1) * coverage) - 1)
    return float(np.sort(np.abs(actual_log - predicted_log))[rank])


def regression_metrics(actual_log: np.ndarray, predicted_log: np.ndarray) -> dict[str, float]:
    actual = np.exp(actual_log)
    predicted = np.exp(predicted_log)
    percentage = (predicted - actual) / actual
    absolute_percentage = np.abs(percentage)
    return {
        "rows": int(len(actual_log)),
        "mae_amount_per_sqm": float(mean_absolute_error(actual, predicted)),
        "median_absolute_percentage_error": float(np.median(absolute_percentage)),
        "p75_absolute_percentage_error": float(np.quantile(absolute_percentage, 0.75)),
        "p90_absolute_percentage_error": float(np.quantile(absolute_percentage, 0.90)),
        "signed_median_percentage_error": float(np.median(percentage)),
        "rmse_log": float(math.sqrt(mean_squared_error(actual_log, predicted_log))),
        "r2_log": float(r2_score(actual_log, predicted_log)),
    }


def interval_metrics(actual_log: np.ndarray, predicted_log: np.ndarray,
                     half_width: float) -> dict[str, float]:
    lower = np.exp(predicted_log - half_width)
    upper = np.exp(predicted_log + half_width)
    widths = upper - lower
    covered = (actual_log >= predicted_log - half_width) & (
        actual_log <= predicted_log + half_width
    )
    return {
        "empirical_interval_coverage": float(np.mean(covered)),
        "mean_interval_width": float(np.mean(widths)),
        "median_interval_width": float(np.median(widths)),
    }


# Backward-compatible helper for callers outside model selection.
def temporal_split_indices(contract_dates: list[date], as_of: date | None = None):
    folds, _, holdout = temporal_folds(
        contract_dates, as_of=as_of, min_training_rows=10,
        min_calibration_rows=10, min_validation_rows=10,
    )
    if holdout is None:
        raise ValueError("temporal validation requires complete calendar years")
    return final_holdout_split(
        contract_dates, holdout, min_training_rows=10,
        min_calibration_rows=10, min_holdout_rows=10,
    )
