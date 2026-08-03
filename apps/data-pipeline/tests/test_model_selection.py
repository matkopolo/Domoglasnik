import json
from datetime import date

import numpy as np

from real_estate_etl.model_selection import (
    SelectionSettings,
    build_pipeline,
    grouped_robustness,
    optimize_model,
)
from real_estate_etl.model_validation import interval_metrics, regression_metrics


def population(seed=4):
    generator = np.random.default_rng(seed)
    rows = []
    target = []
    dates = []
    groups = []
    for year in range(2018, 2027):
        for index in range(35):
            area = generator.uniform(3.5, 5.2)
            municipality = str(index % 8)
            rows.append([area, 1980 + index % 35, year, index % 12 + 1,
                         450000 + index, 100000 + index, "apartment", municipality])
            target.append(5.5 + 0.3 * area + 0.015 * (year - 2018) + generator.normal(0, 0.03))
            dates.append(date(year, index % 12 + 1, 1))
            groups.append(f"building:{index % 14}")
    return np.asarray(rows, dtype=object), np.asarray(target), dates, np.asarray(groups, dtype=object)


def settings():
    return SelectionSettings(min_training_rows=30, min_calibration_rows=30,
                             min_validation_rows=30, random_seed=17)


def configs():
    return [
        {"family": "ridge", "alpha": 0.1},
        {"family": "ridge", "alpha": 2.0},
        {"family": "elastic_net", "alpha": 0.01, "l1_ratio": 0.2},
        {"family": "hist_gradient_boosting", "learning_rate": 0.1,
         "max_iter": 30, "max_leaf_nodes": 15, "min_samples_leaf": 10,
         "l2_regularization": 1.0},
    ]


def test_selection_is_deterministic_and_holdout_targets_cannot_select_hyperparameters():
    features, target, dates, groups = population()
    first = optimize_model(features, target, dates, groups, settings(), configs())
    changed = target.copy()
    changed[np.asarray([value.year == 2025 for value in dates])] += 4
    second = optimize_model(features, changed, dates, groups, settings(), configs())
    assert first["selected_configuration"] == second["selected_configuration"]
    assert first["selected_configuration"] == optimize_model(
        features, target, dates, groups, settings(), configs()
    )["selected_configuration"]
    assert first["final_holdout"]["mae_amount_per_sqm"] != second["final_holdout"]["mae_amount_per_sqm"]
    json.dumps(first)


def test_pipeline_handles_unknown_categories_and_training_only_imputation():
    model = build_pipeline({"family": "ridge", "alpha": 1.0}, 17)
    training = np.asarray([
        [1.0, 1980, 2020, 1, 1, 1, "apartment", "1"],
        [3.0, 2000, 2020, 2, 2, 2, "apartment", "1"],
    ], dtype=object)
    model.fit(training, np.asarray([1.0, 2.0]))
    unknown = np.asarray([[1000.0, None, 2021, 1, 1, 1, "castle", "999"]], dtype=object)
    assert np.isfinite(model.predict(unknown)).all()
    imputer = model.named_steps["features"].named_transformers_["numeric"].named_steps["impute"]
    assert imputer.statistics_[0] == 2.0


def test_grouped_robustness_is_disjoint_and_explicit_when_insufficient():
    features, target, _, groups = population()
    result = grouped_robustness(
        features, target, groups, {"family": "ridge", "alpha": 1.0}, settings()
    )
    assert result["status"] == "evaluated"
    assert result["groups_disjoint"] is True
    assert result["held_out_group_count"] > 0
    insufficient = grouped_robustness(
        features[:10], target[:10], np.asarray(["one"] * 10),
        {"family": "ridge", "alpha": 1.0}, settings(),
    )
    assert insufficient == {"status": "insufficient_groups", "distinct_groups": 1}


def test_metric_and_interval_calculations():
    actual = np.log(np.asarray([100.0, 200.0, 400.0, 800.0]))
    predicted = np.log(np.asarray([110.0, 180.0, 500.0, 400.0]))
    metrics = regression_metrics(actual, predicted)
    assert np.isclose(metrics["mae_amount_per_sqm"], 132.5)
    assert np.isclose(metrics["median_absolute_percentage_error"], 0.175)
    assert np.isclose(metrics["signed_median_percentage_error"], 0.0)
    intervals = interval_metrics(actual, predicted, np.log(1.25))
    assert intervals["empirical_interval_coverage"] == 0.75


def test_insufficient_temporal_folds_has_explicit_status():
    features, target, dates, groups = population()
    result = optimize_model(
        features[:70], target[:70], dates[:70], groups[:70], settings(), configs()
    )
    assert result["status"] == "insufficient_temporal_folds"
    assert result["skipped_folds"]
