"""End-to-end refresh, reset, and scheduling operations for the worker."""

from __future__ import annotations

import os
import signal
import sys
import time
import uuid
from collections.abc import Callable
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from threading import Event, Thread

import psycopg

from .geography import sync_geographies
from .models import RefreshSummary
from .pipeline import refresh_all_national

REFRESH_LOCK_ID = 724_202_607
DEFAULT_INTERVAL_DAYS = 14
DEFAULT_RETRY_HOURS = 1
DEFAULT_ANALYTICS_PROGRESS_INTERVAL_SECONDS = 15.0

ANALYTICS_REFRESH_FUNCTIONS = (
    "analytics.refresh_sale_facts",
    "analytics.refresh_land_transaction_facts",
    "analytics.refresh_sale_outliers",
    "analytics.refresh_rental_facts",
    "analytics.refresh_rental_outliers",
    "analytics.refresh_anomaly_signals",
    "analytics.refresh_near_duplicate_candidates",
    "analytics.refresh_missingness_profiles",
    "analytics.refresh_repeat_sales_indices",
)

RESET_TABLES = (
    "analytics.valuation_predictions",
    "analytics.market_indices",
    "analytics.repeat_sale_pairs",
    "analytics.index_calculation_runs",
    "analytics.model_runs",
    "analytics.anomaly_signals",
    "analytics.near_duplicate_candidates",
    "analytics.missingness_profiles",
    "analytics.land_transaction_facts",
    "analytics.sale_outlier_evaluations",
    "analytics.rental_outlier_evaluations",
    "analytics.sale_facts",
    "analytics.rental_facts",
    "analytics.refresh_runs",
    "public.building_parts",
    "public.land_parcels",
    "public.transactions",
    "public.rental_building_parts",
    "public.rental_transactions",
    "public.codebook_values",
    "public.codebook_imports",
    "public.source_files",
)


class RefreshAlreadyRunning(RuntimeError):
    pass


@dataclass(frozen=True)
class SystemRefreshResult:
    archives: RefreshSummary
    geography_counts: dict[str, int]
    model_rows: dict[str, int]


def _analytics_progress_interval() -> float:
    raw = os.getenv(
        "ANALYTICS_PROGRESS_INTERVAL_SECONDS",
        str(DEFAULT_ANALYTICS_PROGRESS_INTERVAL_SECONDS),
    )
    try:
        return max(1.0, float(raw))
    except ValueError:
        return DEFAULT_ANALYTICS_PROGRESS_INTERVAL_SECONDS


def _log_running_stage(label: str, started: float, done: Event, interval: float) -> None:
    while not done.wait(interval):
        print(
            f"  {label}: PostgreSQL is still processing "
            f"(elapsed={time.monotonic() - started:.1f}s)",
            flush=True,
        )


def _refresh_functions(
    connection,
    functions: tuple[str, ...],
    *,
    progress_interval_seconds: float | None = None,
) -> dict[str, int]:
    counts: dict[str, int] = {}
    interval = progress_interval_seconds or _analytics_progress_interval()
    with connection.transaction(), connection.cursor() as cursor:
        for function in functions:
            label = function.rsplit(".", 1)[-1]
            started = time.monotonic()
            print(f"  {label}: started", flush=True)
            done = Event()
            heartbeat = Thread(
                target=_log_running_stage,
                args=(label, started, done, interval),
                name=f"{label}-progress",
                daemon=True,
            )
            heartbeat.start()
            try:
                cursor.execute(f"SELECT {function}()")  # fixed internal allow-list
            finally:
                done.set()
                heartbeat.join()
            value = cursor.fetchone()[0]
            counts[function] = int(value or 0)
            print(
                f"  {label}: "
                f"{int(value or 0):,} rows in {time.monotonic() - started:.1f}s",
                flush=True,
            )
    return counts


def refresh_analytics(connection, *, database_url: str) -> dict[str, int]:
    """Build one coherent analytical generation in the caller's transaction."""
    del database_url  # retained for CLI/API compatibility
    return _refresh_functions(connection, ANALYTICS_REFRESH_FUNCTIONS)


def _release_status(database_url: str, release_id: uuid.UUID, stage: str) -> None:
    with psycopg.connect(database_url) as status_connection:
        status_connection.execute(
            """UPDATE analytics.analytical_releases SET stage=%s WHERE id=%s""",
            (stage, release_id),
        )


def _start_release(connection, database_url: str) -> uuid.UUID:
    release_id = uuid.uuid4()
    sale_through, rent_through = connection.execute(
        """SELECT (SELECT max(contract_date) FROM transactions),
                  (SELECT max(contract_date) FROM rental_transactions)"""
    ).fetchone()
    connection.commit()
    with psycopg.connect(database_url) as status_connection:
        status_connection.execute(
            """
            UPDATE analytics.analytical_releases
            SET status='failed',stage='superseded',failed_at=clock_timestamp(),
                failure_message='superseded by a newer refresh'
            WHERE status='processing'
            """
        )
        status_connection.execute(
            """
            INSERT INTO analytics.analytical_releases(
                id,status,stage,source_sale_through,source_rent_through
            ) VALUES(%s,'processing','source',%s,%s)
            """,
            (release_id, sale_through, rent_through),
        )
        status_connection.execute(
            """
            UPDATE analytics.analytical_publication
            SET pending_release_id=%s,updated_at=clock_timestamp()
            WHERE singleton
            """,
            (release_id,),
        )
    return release_id


def _update_release_source(connection, database_url: str, release_id: uuid.UUID) -> None:
    sale_through, rent_through = connection.execute(
        """SELECT (SELECT max(contract_date) FROM transactions),
                  (SELECT max(contract_date) FROM rental_transactions)"""
    ).fetchone()
    connection.commit()
    with psycopg.connect(database_url) as status_connection:
        status_connection.execute(
            """UPDATE analytics.analytical_releases
               SET source_sale_through=%s,source_rent_through=%s,stage='geography'
               WHERE id=%s""",
            (sale_through, rent_through, release_id),
        )


def _fail_release(database_url: str, release_id: uuid.UUID, error: Exception) -> None:
    with psycopg.connect(database_url) as status_connection:
        status_connection.execute(
            """
            UPDATE analytics.analytical_releases
            SET status='failed',stage='failed',failed_at=clock_timestamp(),
                failure_message=left(%s,2000)
            WHERE id=%s
            """,
            (str(error), release_id),
        )
        status_connection.execute(
            """
            UPDATE analytics.analytical_publication
            SET pending_release_id=NULL,updated_at=clock_timestamp()
            WHERE singleton AND pending_release_id=%s
            """,
            (release_id,),
        )


def _publish_release(
    connection, release_id: uuid.UUID, model_versions: str | None
) -> None:
    connection.execute(
        """
        UPDATE analytics.analytical_releases
        SET status='published',stage='published',published_at=clock_timestamp(),
            analytics_sale_through=(SELECT max(contract_date) FROM analytics.sale_facts),
            analytics_rent_through=(SELECT max(contract_date) FROM analytics.rental_facts),
            model_versions=%s
        WHERE id=%s
        """,
        (model_versions, release_id),
    )
    connection.execute(
        """
        UPDATE analytics.analytical_publication
        SET active_release_id=%s,pending_release_id=NULL,updated_at=clock_timestamp()
        WHERE singleton
        """,
        (release_id,),
    )


def reset_market_data(connection) -> None:
    """Delete market data while preserving users and installation state."""
    tables = ", ".join(RESET_TABLES)
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute(f"TRUNCATE TABLE {tables} RESTART IDENTITY CASCADE")


def refresh_system(
    connection,
    *,
    output_dir: Path,
    end_year: int,
    force: bool,
    workers: int,
    verbose: bool,
    database_url: str,
) -> SystemRefreshResult:
    """Run source → geography → analytics; model training is explicitly opt-in."""
    acquired = connection.execute(
        "SELECT pg_try_advisory_lock(%s)", (REFRESH_LOCK_ID,)
    ).fetchone()[0]
    connection.commit()
    if not acquired:
        raise RefreshAlreadyRunning("another data-pipeline refresh is already running")

    try:
        return _refresh_system_unlocked(
            connection,
            output_dir=output_dir,
            end_year=end_year,
            force=force,
            workers=workers,
            verbose=verbose,
            database_url=database_url,
        )
    finally:
        connection.rollback()
        connection.execute("SELECT pg_advisory_unlock(%s)", (REFRESH_LOCK_ID,))
        connection.commit()


def reset_and_repopulate(
    connection,
    *,
    output_dir: Path,
    end_year: int,
    workers: int,
    verbose: bool,
    database_url: str,
) -> SystemRefreshResult:
    acquired = connection.execute(
        "SELECT pg_try_advisory_lock(%s)", (REFRESH_LOCK_ID,)
    ).fetchone()[0]
    connection.commit()
    if not acquired:
        raise RefreshAlreadyRunning("another data-pipeline refresh is already running")
    try:
        reset_market_data(connection)
        print("Removed imported and derived market data; users and preferences were preserved.")
        return _refresh_system_unlocked(
            connection,
            output_dir=output_dir,
            end_year=end_year,
            force=False,
            workers=workers,
            verbose=verbose,
            database_url=database_url,
        )
    finally:
        connection.rollback()
        connection.execute("SELECT pg_advisory_unlock(%s)", (REFRESH_LOCK_ID,))
        connection.commit()


def _refresh_system_unlocked(
    connection,
    *,
    output_dir: Path,
    end_year: int,
    force: bool,
    workers: int,
    verbose: bool,
    database_url: str,
) -> SystemRefreshResult:
    release_id = _start_release(connection, database_url)
    try:
        archives = refresh_all_national(
            connection,
            output_dir,
            end_year,
            force,
            workers,
            verbose,
            refresh_current_year=True,
        )
        _update_release_source(connection, database_url, release_id)
        geography_counts = sync_geographies(connection)
        _release_status(database_url, release_id, "analytics")
        bootstrap_required = connection.execute(
            """
            SELECT model_bootstrap_completed_at IS NULL
            FROM analytics.pipeline_state WHERE singleton
            """
        ).fetchone()[0]
        connection.commit()
        model_rows: dict[str, int] = {}
        with connection.transaction():
            analytics_counts = refresh_analytics(connection, database_url=database_url)
            print(
                "Refreshed analytics: "
                + ", ".join(
                    f"{name.rsplit('.', 1)[-1]}={count}"
                    for name, count in analytics_counts.items()
                )
            )
            should_train = bootstrap_required or archives.imported > 0
            model_versions = connection.execute(
                """SELECT model_bootstrap_version FROM analytics.pipeline_state
                   WHERE singleton"""
            ).fetchone()[0]
            if should_train:
                _release_status(database_url, release_id, "models")
                print(
                    "Starting valuation-model training for the pending analytical release.",
                    flush=True,
                )
                from .model_reporting import format_model_report
                from .modeling import refresh_models

                model_results = refresh_models(connection, workers)
                for result in model_results:
                    print(format_model_report(result))
                model_rows = {
                    result.transaction_kind: result.rows for result in model_results
                }
                model_versions = ",".join(
                    result.model_version for result in model_results
                )
            else:
                print(
                    "No new GURS archives; retaining the currently published model."
                )
            _publish_release(connection, release_id, model_versions)
        return SystemRefreshResult(archives, geography_counts, model_rows)
    except Exception as error:
        connection.rollback()
        _fail_release(database_url, release_id, error)
        raise


def scheduled_refresh_loop(
    database_url: str,
    *,
    output_dir: Path,
    interval_days: int = DEFAULT_INTERVAL_DAYS,
    retry_hours: int = DEFAULT_RETRY_HOURS,
    workers: int,
    verbose: bool,
    health_file: Path | None = None,
    stop_event: Event | None = None,
    refresh: Callable[..., SystemRefreshResult] = refresh_system,
) -> None:
    """Run immediately, then every interval; retry failures without losing last-good health."""
    if interval_days < 1:
        raise ValueError("interval_days must be at least 1")
    if retry_hours < 1:
        raise ValueError("retry_hours must be at least 1")
    stop_event = stop_event or Event()

    def request_stop(_signal, _frame) -> None:
        stop_event.set()

    if stop_event is not None and os.getpid() == 1:
        signal.signal(signal.SIGTERM, request_stop)
        signal.signal(signal.SIGINT, request_stop)

    while not stop_event.is_set():
        try:
            with psycopg.connect(database_url) as connection:
                refresh(
                    connection,
                    output_dir=output_dir,
                    end_year=date.today().year,
                    force=False,
                    workers=workers,
                    verbose=verbose,
                    database_url=database_url,
                )
            if health_file:
                health_file.parent.mkdir(parents=True, exist_ok=True)
                health_file.touch()
            wait_seconds = interval_days * 24 * 60 * 60
            print(f"Scheduled refresh succeeded; next run in {interval_days} days.")
        except Exception as exc:
            print(f"Scheduled refresh failed: {exc}", file=sys.stderr)
            wait_seconds = retry_hours * 60 * 60
            print(f"Retrying scheduled refresh in {retry_hours} hour(s).", file=sys.stderr)
        stop_event.wait(wait_seconds)
