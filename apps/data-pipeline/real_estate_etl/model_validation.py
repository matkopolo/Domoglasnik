"""Pure temporal validation, scoring, interval, and metric calculations."""

from __future__ import annotations

import math
from datetime import date

import numpy as np
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score

NOMINAL_INTERVAL_COVERAGE = 0.90


def percentile_scores(values: np.ndarray) -> np.ndarray:
    if len(values) == 0:
        return np.array([], dtype=float)
    ordered = np.sort(values)
    return np.searchsorted(ordered, values, side="right") / len(values)


def temporal_split_indices(
    contract_dates: list[date], as_of: date | None = None
) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    if len(contract_dates) < 30:
        raise ValueError("temporal validation requires at least 30 rows")
    as_of = as_of or date.today()
    complete_years = sorted(
        {value.year for value in contract_dates if value.year < as_of.year}
    )
    if len(complete_years) >= 2:
        test_year, calibration_year = complete_years[-1], complete_years[-2]
        training = np.asarray(
            [i for i, value in enumerate(contract_dates) if value.year < calibration_year],
            dtype=int,
        )
        calibration = np.asarray(
            [i for i, value in enumerate(contract_dates) if value.year == calibration_year],
            dtype=int,
        )
        test = np.asarray(
            [i for i, value in enumerate(contract_dates) if value.year == test_year],
            dtype=int,
        )
        if min(len(training), len(calibration), len(test)) >= 10:
            return training, calibration, test

    order = np.argsort(np.asarray(contract_dates, dtype="datetime64[D]"), kind="stable")
    test_size = max(10, len(order) // 5)
    calibration_size = max(10, len(order) // 5)
    if test_size + calibration_size >= len(order):
        raise ValueError("not enough rows for disjoint temporal splits")
    return (
        order[: -(test_size + calibration_size)],
        order[-(test_size + calibration_size) : -test_size],
        order[-test_size:],
    )


def interval_half_width(
    actual_log: np.ndarray,
    predicted_log: np.ndarray,
    coverage: float = NOMINAL_INTERVAL_COVERAGE,
) -> float:
    if len(actual_log) == 0 or len(actual_log) != len(predicted_log):
        raise ValueError("calibration arrays must be non-empty and equal length")
    rank = min(len(actual_log) - 1, math.ceil((len(actual_log) + 1) * coverage) - 1)
    return float(np.sort(np.abs(actual_log - predicted_log))[rank])


def regression_metrics(
    actual_log: np.ndarray, predicted_log: np.ndarray
) -> dict[str, float]:
    actual = np.exp(actual_log)
    predicted = np.exp(predicted_log)
    return {
        "rows": int(len(actual_log)),
        "mae_amount_per_sqm": float(mean_absolute_error(actual, predicted)),
        "median_absolute_percentage_error": float(
            np.median(np.abs(actual - predicted) / actual)
        ),
        "rmse_log": float(math.sqrt(mean_squared_error(actual_log, predicted_log))),
        "r2_log": float(r2_score(actual_log, predicted_log)),
    }
