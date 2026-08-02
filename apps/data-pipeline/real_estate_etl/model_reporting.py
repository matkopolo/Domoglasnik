"""Human-readable validation reporting for offline valuation-model refreshes."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any


def format_model_report(result: Any) -> str:
    """Format persisted model metrics without implying a single accuracy score."""
    metrics: Mapping[str, Any] = result.metrics
    kind = "sale price/m²" if result.transaction_kind == "sale" else "rent/m²"
    lines = [
        f"Model validation — {kind}",
        f"  version={result.model_version} rows={result.rows:,} "
        f"elapsed={_duration(result.elapsed_seconds)}",
        "  estimators: hedonic valuation=Ridge regression; "
        "anomaly signal=Isolation Forest",
        "  latest-complete-year temporal holdout: "
        f"n={int(metrics.get('rows', 0)):,}, "
        f"MAE={_number(metrics.get('mae_amount_per_sqm'))}, "
        f"median APE={_percent(metrics.get('median_absolute_percentage_error'))}, "
        f"log RMSE={_number(metrics.get('rmse_log'), 3)}, "
        f"log R²={_number(metrics.get('r2_log'), 3)}",
        "  split-conformal interval: "
        f"nominal={_percent(metrics.get('nominal_interval_coverage'))}, "
        f"observed={_percent(metrics.get('empirical_interval_coverage'))}",
    ]

    groups = metrics.get("property_groups", {})
    if isinstance(groups, Mapping) and groups:
        lines.append("  property-group diagnostics:")
        for group, raw in sorted(groups.items()):
            if not isinstance(raw, Mapping):
                continue
            temporal = raw.get("temporal", {})
            spatial = raw.get("spatial", {})
            if not isinstance(temporal, Mapping) or not isinstance(spatial, Mapping):
                continue
            model_mae = _as_float(temporal.get("mae_amount_per_sqm"))
            baseline_mae = _as_float(temporal.get("local_median_baseline_mae"))
            improvement = (
                1 - model_mae / baseline_mae
                if model_mae is not None and baseline_mae and baseline_mae > 0
                else None
            )
            spatial_summary = (
                f"spatial R²={_number(spatial.get('r2_log'), 3)} "
                f"across {int(spatial.get('held_out_municipalities', 0))} held-out municipalities"
                if "r2_log" in spatial
                else f"spatial={spatial.get('status', 'unavailable')}"
            )
            lines.append(
                f"    {group}: test n={int(temporal.get('rows', 0)):,}, "
                f"MAE={_number(model_mae)}, median APE="
                f"{_percent(temporal.get('median_absolute_percentage_error'))}, "
                f"log R²={_number(temporal.get('r2_log'), 3)}, "
                f"vs local-median baseline={_signed_percent(improvement)}; "
                f"{spatial_summary}"
            )

    warnings = _warnings(metrics)
    for warning in warnings:
        lines.append(f"  WARNING: {warning}")
    lines.append(
        "  Interpretation: validation error and coverage are diagnostics, not a "
        "certified appraisal or a classification accuracy."
    )
    return "\n".join(lines)


def _warnings(metrics: Mapping[str, Any]) -> list[str]:
    warnings: list[str] = []
    coverage = _as_float(metrics.get("empirical_interval_coverage"))
    nominal = _as_float(metrics.get("nominal_interval_coverage"))
    if coverage is not None and nominal is not None and coverage < nominal - 0.03:
        warnings.append("prediction-interval coverage is more than 3 percentage points below nominal")
    groups = metrics.get("property_groups", {})
    if isinstance(groups, Mapping):
        weak = []
        for group, raw in groups.items():
            if not isinstance(raw, Mapping):
                continue
            temporal = raw.get("temporal", {})
            spatial = raw.get("spatial", {})
            if isinstance(temporal, Mapping) and (_as_float(temporal.get("r2_log")) or 0) < 0:
                weak.append(f"{group} temporal")
            if isinstance(spatial, Mapping) and "r2_log" in spatial and (
                _as_float(spatial.get("r2_log")) or 0
            ) < 0:
                weak.append(f"{group} spatial")
        if weak:
            warnings.append("negative log R² (worse than a mean baseline): " + ", ".join(weak))
    return warnings


def _as_float(value: Any) -> float | None:
    return float(value) if isinstance(value, (int, float)) else None


def _number(value: Any, decimals: int = 2) -> str:
    parsed = _as_float(value)
    return "n/a" if parsed is None else f"{parsed:,.{decimals}f}"


def _percent(value: Any) -> str:
    parsed = _as_float(value)
    return "n/a" if parsed is None else f"{100 * parsed:.1f}%"


def _signed_percent(value: float | None) -> str:
    return "n/a" if value is None else f"{100 * value:+.1f}% MAE improvement"


def _duration(seconds: float) -> str:
    minutes, remaining = divmod(round(seconds), 60)
    return f"{minutes}m {remaining:02d}s" if minutes else f"{remaining}s"
