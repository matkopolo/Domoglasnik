from types import SimpleNamespace

from real_estate_etl.model_reporting import format_model_report


def test_model_report_explains_temporal_spatial_baseline_and_coverage():
    result = SimpleNamespace(
        transaction_kind="sale",
        rows=1_000,
        model_version="sale-v1",
        elapsed_seconds=65,
        metrics={
            "rows": 100,
            "mae_amount_per_sqm": 420.5,
            "median_absolute_percentage_error": 0.18,
            "rmse_log": 0.31,
            "r2_log": 0.62,
            "nominal_interval_coverage": 0.9,
            "empirical_interval_coverage": 0.88,
            "property_groups": {
                "apartment": {
                    "temporal": {
                        "rows": 80,
                        "mae_amount_per_sqm": 400.0,
                        "median_absolute_percentage_error": 0.17,
                        "r2_log": 0.64,
                        "local_median_baseline_mae": 500.0,
                    },
                    "spatial": {"r2_log": 0.41, "held_out_municipalities": 12},
                }
            },
        },
    )

    report = format_model_report(result)

    assert "Model validation — sale price/m²" in report
    assert "Ridge regression" in report
    assert "Isolation Forest" in report
    assert "MAE=420.50" in report
    assert "median APE=18.0%" in report
    assert "observed=88.0%" in report
    assert "+20.0% MAE improvement" in report
    assert "12 held-out municipalities" in report
    assert "not a certified appraisal" in report


def test_model_report_warns_about_negative_r_squared_and_low_coverage():
    result = SimpleNamespace(
        transaction_kind="rent",
        rows=100,
        model_version="rent-v1",
        elapsed_seconds=1,
        metrics={
            "rows": 20,
            "nominal_interval_coverage": 0.9,
            "empirical_interval_coverage": 0.8,
            "property_groups": {
                "apartment": {
                    "temporal": {"rows": 10, "r2_log": -0.1},
                    "spatial": {"r2_log": -0.2, "held_out_municipalities": 3},
                }
            },
        },
    )

    report = format_model_report(result)

    assert "coverage is more than 3 percentage points below nominal" in report
    assert "apartment temporal, apartment spatial" in report
