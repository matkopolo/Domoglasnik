from pathlib import Path
from threading import Event
from time import sleep
from unittest.mock import MagicMock, patch

import pytest

from real_estate_etl.cli import build_parser, validate_arguments
from real_estate_etl.models import RefreshSummary
from real_estate_etl.operations import (
    ANALYTICS_REFRESH_FUNCTIONS,
    ANALYTICS_STAGE_DESCRIPTIONS,
    SystemRefreshResult,
    _refresh_functions,
    _refresh_system_unlocked,
    refresh_analytics,
    refresh_system,
    reset_market_data,
    scheduled_refresh_loop,
)


def test_long_analytics_refresh_emits_periodic_heartbeat(capsys):
    connection = MagicMock()
    cursor = connection.cursor.return_value.__enter__.return_value
    cursor.fetchone.return_value = (456_585,)
    cursor.execute.side_effect = lambda _statement: sleep(0.035)

    counts = _refresh_functions(
        connection,
        ("analytics.refresh_rental_facts",),
        progress_interval_seconds=0.01,
    )

    assert counts == {"analytics.refresh_rental_facts": 456_585}
    output = capsys.readouterr().out
    assert "refresh_rental_facts: started" in output
    assert "one quality-audited analytical fact per rental agreement" in output
    assert output.count("PostgreSQL is still processing") >= 2
    assert "refresh_rental_facts: 456,585 rows in" in output


def test_every_analytics_stage_has_a_non_destructive_description():
    assert set(ANALYTICS_STAGE_DESCRIPTIONS) == set(ANALYTICS_REFRESH_FUNCTIONS)
    assert "marked, not deleted" in ANALYTICS_STAGE_DESCRIPTIONS[
        "analytics.refresh_sale_outliers"
    ]


def test_scheduler_defaults_to_every_fourteen_days():
    parser = build_parser()
    args = parser.parse_args(["schedule"])
    validate_arguments(parser, args)

    assert args.interval_days == 14
    assert args.retry_hours == 1


def test_reset_requires_explicit_confirmation():
    parser = build_parser()
    args = parser.parse_args(["reset-and-repopulate"])

    with pytest.raises(SystemExit):
        validate_arguments(parser, args)


def test_refresh_analytics_uses_documented_dependency_order(capsys):
    connection = MagicMock()
    cursor = connection.cursor.return_value.__enter__.return_value
    cursor.fetchone.return_value = (3,)
    counts = refresh_analytics(connection, database_url="postgresql://test")

    assert [call.args[0] for call in cursor.execute.call_args_list] == [
        f"SELECT {function}()" for function in ANALYTICS_REFRESH_FUNCTIONS
    ]
    assert counts == {function: 3 for function in ANALYTICS_REFRESH_FUNCTIONS}
    output = capsys.readouterr().out
    for function in ANALYTICS_REFRESH_FUNCTIONS:
        assert f"{function.rsplit('.', 1)[-1]}: 3 rows in " in output


def test_refresh_analytics_stops_generation_when_a_stage_fails():
    connection = MagicMock()
    cursor = connection.cursor.return_value.__enter__.return_value

    def execute(statement):
        if "refresh_sale_facts" in statement:
            raise RuntimeError("sale refresh failed")
        raise AssertionError("later analytical stage ran after failure")

    cursor.execute.side_effect = execute
    with pytest.raises(RuntimeError, match="sale refresh failed"):
        refresh_analytics(connection, database_url="postgresql://test")

    assert cursor.execute.call_count == 1


def test_reset_market_data_preserves_identity_tables():
    connection = MagicMock()
    cursor = connection.cursor.return_value.__enter__.return_value

    reset_market_data(connection)

    statement = cursor.execute.call_args.args[0]
    assert "public.source_files" in statement
    assert "analytics.model_runs" in statement
    assert "app_users" not in statement
    assert "user_preferences" not in statement
    assert "RESTART IDENTITY CASCADE" in statement


def test_refresh_system_holds_and_releases_advisory_lock(tmp_path: Path):
    connection = MagicMock()
    connection.execute.return_value.fetchone.return_value = (True,)
    expected = SystemRefreshResult(RefreshSummary(1, 2, 0), {"country": 1}, {"sale": 10})

    with patch(
        "real_estate_etl.operations._refresh_system_unlocked", return_value=expected
    ) as run:
        result = refresh_system(
            connection,
            output_dir=tmp_path,
            end_year=2026,
            force=False,
            workers=2,
            verbose=False,
            database_url="postgresql://test",
        )

    assert result == expected
    run.assert_called_once()
    assert "pg_try_advisory_lock" in connection.execute.call_args_list[0].args[0]
    assert "pg_advisory_unlock" in connection.execute.call_args_list[-1].args[0]


def test_system_refresh_skips_models_after_first_install(tmp_path: Path, capsys):
    connection = MagicMock()
    connection.execute.return_value.fetchone.return_value = (False,)
    archives = RefreshSummary(0, 3, 0)
    with (
        patch("real_estate_etl.operations.refresh_all_national", return_value=archives),
        patch("real_estate_etl.operations.sync_geographies", return_value={"country": 1}),
        patch("real_estate_etl.operations.refresh_analytics", return_value={"facts": 3}),
        patch("real_estate_etl.operations._start_release", return_value=MagicMock()),
        patch("real_estate_etl.operations._update_release_source"),
        patch("real_estate_etl.operations._release_status"),
        patch("real_estate_etl.operations._publish_release"),
        patch("real_estate_etl.modeling.refresh_models") as train,
    ):
        result = _refresh_system_unlocked(
            connection,
            output_dir=tmp_path,
            end_year=2026,
            force=False,
            workers=4,
            verbose=False,
            database_url="postgresql://test",
        )

    train.assert_not_called()
    assert result.model_rows == {}
    assert "No new GURS archives" in capsys.readouterr().out


@pytest.mark.parametrize("bootstrap_required", [True, False])
def test_system_refresh_trains_models_for_bootstrap_or_new_archive(
    tmp_path: Path, capsys, bootstrap_required: bool
):
    connection = MagicMock()
    connection.execute.return_value.fetchone.return_value = (bootstrap_required,)
    archives = RefreshSummary(1, 2, 0)
    model_result = MagicMock(
        transaction_kind="sale", rows=12, model_version="sale-test"
    )
    with (
        patch("real_estate_etl.operations.refresh_all_national", return_value=archives),
        patch("real_estate_etl.operations.sync_geographies", return_value={"country": 1}),
        patch("real_estate_etl.operations.refresh_analytics", return_value={"facts": 3}),
        patch("real_estate_etl.operations._start_release", return_value=MagicMock()),
        patch("real_estate_etl.operations._update_release_source"),
        patch("real_estate_etl.operations._release_status"),
        patch("real_estate_etl.operations._publish_release"),
        patch("real_estate_etl.modeling.refresh_models", return_value=[model_result]) as train,
        patch("real_estate_etl.model_reporting.format_model_report", return_value="report"),
    ):
        result = _refresh_system_unlocked(
            connection,
            output_dir=tmp_path,
            end_year=2026,
            force=False,
            workers=4,
            verbose=False,
            database_url="postgresql://test",
        )

    train.assert_called_once_with(connection, 4)
    assert result.model_rows == {"sale": 12}
    assert "pending analytical release" in capsys.readouterr().out


def test_scheduled_refresh_runs_immediately_and_marks_health(tmp_path: Path):
    stop = Event()
    connection = MagicMock()
    context = MagicMock()
    context.__enter__.return_value = connection
    health = tmp_path / "healthy"

    def refresh(*_args, **_kwargs):
        stop.set()
        return SystemRefreshResult(RefreshSummary(0, 34, 0), {}, {})

    with patch("real_estate_etl.operations.psycopg.connect", return_value=context):
        scheduled_refresh_loop(
            "postgresql://example",
            output_dir=tmp_path,
            workers=2,
            verbose=False,
            health_file=health,
            stop_event=stop,
            refresh=refresh,
        )

    assert health.is_file()
