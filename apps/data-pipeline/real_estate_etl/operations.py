"""End-to-end refresh, reset, and scheduling operations for the worker."""

from __future__ import annotations

import os
import signal
import sys
import time
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from threading import Event

import psycopg

from .geography import sync_geographies
from .models import RefreshSummary
from .pipeline import refresh_all_national

REFRESH_LOCK_ID = 724_202_607
DEFAULT_INTERVAL_DAYS = 14
DEFAULT_RETRY_HOURS = 1

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

SALE_ANALYTICS_REFRESH_FUNCTIONS = ANALYTICS_REFRESH_FUNCTIONS[:3]
RENTAL_ANALYTICS_REFRESH_FUNCTIONS = ANALYTICS_REFRESH_FUNCTIONS[3:5]
SHARED_ANALYTICS_REFRESH_FUNCTIONS = ANALYTICS_REFRESH_FUNCTIONS[5:]

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


def _refresh_functions(connection, functions: tuple[str, ...]) -> dict[str, int]:
    counts: dict[str, int] = {}
    with connection.transaction(), connection.cursor() as cursor:
        for function in functions:
            started = time.monotonic()
            cursor.execute(f"SELECT {function}()")  # fixed internal allow-list
            value = cursor.fetchone()[0]
            counts[function] = int(value or 0)
            print(
                f"  {function.rsplit('.', 1)[-1]}: "
                f"{int(value or 0):,} rows in {time.monotonic() - started:.1f}s",
                flush=True,
            )
    return counts


def _refresh_branch(database_url: str, functions: tuple[str, ...]) -> dict[str, int]:
    with psycopg.connect(database_url) as connection:
        return _refresh_functions(connection, functions)


def refresh_analytics(connection, *, database_url: str) -> dict[str, int]:
    """Refresh independent sale/rent branches, then their shared dependants."""
    with ThreadPoolExecutor(max_workers=2, thread_name_prefix="analytics") as executor:
        sale_future = executor.submit(
            _refresh_branch, database_url, SALE_ANALYTICS_REFRESH_FUNCTIONS
        )
        rental_future = executor.submit(
            _refresh_branch, database_url, RENTAL_ANALYTICS_REFRESH_FUNCTIONS
        )
        sale_counts = sale_future.result()
        rental_counts = rental_future.result()

    shared_counts = _refresh_functions(connection, SHARED_ANALYTICS_REFRESH_FUNCTIONS)
    return sale_counts | rental_counts | shared_counts


def reset_market_data(connection) -> None:
    """Delete imported and derived market data while preserving users and preferences."""
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
    """Run one mutually exclusive source → geography → analytics → model refresh."""
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
    archives = refresh_all_national(
        connection,
        output_dir,
        end_year,
        force,
        workers,
        verbose,
        refresh_current_year=True,
    )
    geography_counts = sync_geographies(connection)
    analytics_counts = refresh_analytics(connection, database_url=database_url)
    print(
        "Refreshed analytics: "
        + ", ".join(
            f"{name.rsplit('.', 1)[-1]}={count}"
            for name, count in analytics_counts.items()
        )
    )

    from .model_reporting import format_model_report
    from .modeling import refresh_models

    model_results = refresh_models(connection, workers)
    model_rows = {result.transaction_kind: result.rows for result in model_results}
    for result in model_results:
        print(format_model_report(result))
    return SystemRefreshResult(archives, geography_counts, model_rows)


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
