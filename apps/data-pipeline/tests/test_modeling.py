import numpy as np

from real_estate_etl.modeling import percentile_scores


def test_percentile_scores_are_bounded_and_ranked():
    scores = percentile_scores(np.asarray([10.0, 1.0, 5.0]))
    assert scores.tolist() == [1.0, 1 / 3, 2 / 3]


def test_percentile_scores_handle_empty_input():
    assert percentile_scores(np.asarray([])).size == 0
