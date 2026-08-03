from datetime import date, timedelta

import numpy as np

from real_estate_etl.model_validation import (
    interval_half_width,
    percentile_scores,
    temporal_folds,
    temporal_split_indices,
)


def test_percentile_scores_are_bounded_and_ranked():
    scores = percentile_scores(np.asarray([10.0, 1.0, 5.0]))
    assert scores.tolist() == [1.0, 1 / 3, 2 / 3]


def test_percentile_scores_handle_empty_input():
    assert percentile_scores(np.asarray([])).size == 0


def test_temporal_split_is_deterministic_disjoint_and_has_no_future_leakage():
    dates = [date(year, 1, 1) + timedelta(days=index) for year in range(2019, 2026) for index in range(12)]
    first = temporal_split_indices(dates, as_of=date(2026, 8, 1))
    second = temporal_split_indices(dates, as_of=date(2026, 8, 1))
    assert all(np.array_equal(left, right) for left, right in zip(first, second))
    training, calibration, test = first
    assert set(training).isdisjoint(calibration)
    assert set(training).isdisjoint(test)
    assert set(calibration).isdisjoint(test)
    assert max(dates[index] for index in training) < min(
        dates[index] for index in calibration
    )
    assert max(dates[index] for index in calibration) < min(
        dates[index] for index in test
    )


def test_temporal_split_rejects_small_population():
    with np.testing.assert_raises_regex(ValueError, "no earlier calibration year"):
        temporal_split_indices([date(2025, 1, 1)] * 29)


def test_temporal_split_uses_latest_complete_year_and_prior_calibration_year():
    dates = [
        *[date(2023, 6, 1)] * 20,
        *[date(2024, 6, 1)] * 10,
        *[date(2025, 6, 1)] * 10,
        *[date(2026, 1, 1)] * 10,
    ]
    training, calibration, test = temporal_split_indices(
        dates, as_of=date(2026, 7, 1)
    )
    assert {dates[index].year for index in training} == {2023}
    assert {dates[index].year for index in calibration} == {2024}
    assert {dates[index].year for index in test} == {2025}


def test_rolling_folds_exclude_current_year_and_reserve_latest_complete_year():
    dates = [date(year, 6, 1) for year in range(2018, 2027) for _ in range(10)]
    folds, skipped, holdout = temporal_folds(
        dates, as_of=date(2026, 8, 1), min_training_rows=10,
        min_calibration_rows=10, min_validation_rows=10,
    )
    assert not skipped
    assert holdout == 2025
    assert folds
    for fold in folds:
        assert max(fold.train_years) < fold.calibration_year < fold.validation_year < holdout
        assert set(fold.training).isdisjoint(fold.calibration)
        assert set(fold.training).isdisjoint(fold.validation)
        assert set(fold.calibration).isdisjoint(fold.validation)
        assert all(dates[index].year != 2026 for index in np.concatenate(
            [fold.training, fold.calibration, fold.validation]
        ))


def test_interval_half_width_uses_finite_sample_conformal_rank():
    actual = np.asarray([0.0, 1.0, 2.0, 3.0])
    predicted = np.zeros(4)
    assert interval_half_width(actual, predicted, coverage=0.5) == 2.0


def test_interval_coverage_is_evaluated_on_separate_test_values():
    calibration_actual = np.asarray([0.0, 0.1, -0.1, 0.2, -0.2] * 10)
    width = interval_half_width(calibration_actual, np.zeros(50), coverage=0.9)
    test_actual = np.asarray([0.0, 0.1, 1.0, -1.0])
    covered = np.abs(test_actual) <= width
    assert covered.tolist() == [True, True, False, False]
