from pathlib import Path
from threading import Barrier, Event
from unittest.mock import MagicMock, patch

import pytest

from real_estate_etl.cli import build_parser, validate_arguments
from real_estate_etl.models import RefreshSummary
from real_estate_etl.operations import (
    ANALYTICS_REFRESH_FUNCTIONS,
    RENTAL_ANALYTICS_REFRESH_FUNCTIONS,
    SALE_ANALYTICS_REFRESH_FUNCTIONS,
    SHARED_ANALYTICS_REFRESH_FUNCTIONS,
    SystemRefreshResult,
    refresh_analytics,
    refresh_system,
    reset_market_data,
    scheduled_refresh_loop,
)


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
    barrier = Barrier(2)

    def branch(_database_url, functions):
        barrier.wait(timeout=1)
        return {function: 3 for function in functions}

    with patch("real_estate_etl.operations._refresh_branch", side_effect=branch) as run:
        counts = refresh_analytics(connection, database_url="postgresql://test")

    assert [call.args[0] for call in cursor.execute.call_args_list] == [
        f"SELECT {function}()" for function in SHARED_ANALYTICS_REFRESH_FUNCTIONS
    ]
    assert {call.args for call in run.call_args_list} == {
        ("postgresql://test", SALE_ANALYTICS_REFRESH_FUNCTIONS),
        ("postgresql://test", RENTAL_ANALYTICS_REFRESH_FUNCTIONS),
    }
    assert counts == {function: 3 for function in ANALYTICS_REFRESH_FUNCTIONS}
    output = capsys.readouterr().out
    for function in SHARED_ANALYTICS_REFRESH_FUNCTIONS:
        assert f"{function.rsplit('.', 1)[-1]}: 3 rows in " in output


def test_refresh_analytics_does_not_run_shared_stages_when_a_branch_fails():
    connection = MagicMock()

    def branch(_database_url, functions):
        if functions == SALE_ANALYTICS_REFRESH_FUNCTIONS:
            raise RuntimeError("sale refresh failed")
        return {function: 3 for function in functions}

    with (
        patch("real_estate_etl.operations._refresh_branch", side_effect=branch),
        pytest.raises(RuntimeError, match="sale refresh failed"),
    ):
        refresh_analytics(connection, database_url="postgresql://test")

    connection.cursor.assert_not_called()


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
