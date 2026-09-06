"""Download orchestration for incremental GURS archive refreshes."""

from __future__ import annotations

import logging
import time
import zipfile
from collections.abc import Callable, Iterable
from concurrent.futures import FIRST_COMPLETED, Future, ThreadPoolExecutor, wait
from pathlib import Path
from typing import TypeVar

from .api import DatasetUnavailable, client_for
from .importer import import_zip
from .models import ArchiveResult, RefreshSummary
from .settings import API_BASE, AVAILABLE_THREADS, NATIONAL_DATASETS

logger = logging.getLogger(__name__)

WorkItem = TypeVar("WorkItem")


def sync_reference_data(connection) -> None:
    """Upsert municipality and cadastral-municipality reference data."""
    client = client_for(Path("data/raw"))
    municipalities = client.get_json(f"{API_BASE}/municipalities")
    cadastral = client.get_json(f"{API_BASE}/register-municipalities")
    with connection.cursor() as cursor:
        cursor.executemany(
            """INSERT INTO municipalities(code,name) VALUES (%s,%s)
               ON CONFLICT(code) DO UPDATE SET name=excluded.name""",
            [(int(item["sifra"]), item["name"]) for item in municipalities],
        )
        cursor.executemany(
            """INSERT INTO cadastral_municipalities(code,name) VALUES (%s,%s)
               ON CONFLICT(code) DO UPDATE SET name=excluded.name""",
            [(int(item["sifko"]), item["name"]) for item in cadastral],
        )
    connection.commit()
    logger.info(
        "Synced %s municipalities and %s cadastral municipalities",
        len(municipalities),
        len(cadastral),
    )


def fetch_zip(municipality: int, year: int, output_dir: Path) -> Path:
    return client_for(output_dir).fetch_municipality(municipality, year)


def fetch_national_zip(dataset_type: str, year: int, output_dir: Path) -> Path:
    return client_for(output_dir).fetch_national(dataset_type, year)


def prepare_archive(municipality: int, year: int, output_dir: Path, force: bool) -> ArchiveResult:
    started_at = time.monotonic()
    local = sorted(output_dir.glob(f"ETN_{municipality:03d}_{year}_KPP_*.zip"), reverse=True)
    archive = next((path for path in local if zipfile.is_zipfile(path)), None)
    if archive is not None and not force:
        return ArchiveResult(archive, True, time.monotonic() - started_at)
    return ArchiveResult(fetch_zip(municipality, year, output_dir), False, time.monotonic() - started_at)


def prepare_national_archive(dataset_type: str, year: int, output_dir: Path, force: bool) -> ArchiveResult:
    started_at = time.monotonic()
    code = NATIONAL_DATASETS[dataset_type].archive_code
    local = sorted(output_dir.glob(f"ETN_SLO_{year}_{code}*.zip"), reverse=True)
    archive = next((path for path in local if zipfile.is_zipfile(path)), None)
    if archive is not None and not force:
        return ArchiveResult(archive, True, time.monotonic() - started_at)
    return ArchiveResult(fetch_national_zip(dataset_type, year, output_dir), False, time.monotonic() - started_at)


def format_duration(seconds: float) -> str:
    total_seconds = int(seconds)
    hours, remainder = divmod(total_seconds, 3600)
    minutes, seconds = divmod(remainder, 60)
    if hours:
        return f"{hours}h {minutes}m {seconds}s"
    if minutes:
        return f"{minutes}m {seconds}s"
    return f"{seconds}s"


def validate_worker_count(workers: int, available_threads: int = AVAILABLE_THREADS) -> None:
    if workers < 1:
        raise ValueError("--workers must be at least 1")
    if workers > available_threads:
        raise ValueError(
            f"--workers cannot exceed the {available_threads} logical CPU threads available on this machine"
        )


def _run_refresh(
    connection,
    combinations: Iterable[WorkItem],
    *,
    skipped: int,
    workers: int,
    verbose: bool,
    thread_name: str,
    prepare: Callable[[WorkItem], ArchiveResult],
    label: Callable[[int, int, WorkItem], str],
    summary_prefix: str,
) -> RefreshSummary:
    """Download concurrently while importing sequentially on one DB connection."""
    started_at = time.monotonic()
    work = iter(combinations)
    total = len(combinations) if hasattr(combinations, "__len__") else 0
    imported = unavailable = 0
    already_imported = skipped
    download_seconds = parsing_seconds = database_seconds = 0.0
    logger.info(
        f"Queued {total} archives; skipped {skipped} already imported; "
        f"download workers={workers}; available logical CPU threads={AVAILABLE_THREADS}"
    )

    executor = ThreadPoolExecutor(max_workers=workers, thread_name_prefix=thread_name)
    pending: dict[Future[ArchiveResult], WorkItem] = {}

    def submit_next() -> bool:
        try:
            item = next(work)
        except StopIteration:
            return False
        pending[executor.submit(prepare, item)] = item
        return True

    for _ in range(min(total, workers * 2)):
        submit_next()

    position = 0
    try:
        while pending:
            completed, _ = wait(pending, return_when=FIRST_COMPLETED)
            for future in completed:
                item = pending.pop(future)
                position += 1
                item_label = label(position, total, item)
                submit_next()
                try:
                    archive = future.result()
                    download_seconds += archive.download_seconds
                    if archive.reused:
                        logger.info("%s: using existing %s", item_label, archive.path)
                    elif verbose:
                        logger.info("%s: importing %s", item_label, archive.path)
                    stats = import_zip(connection, archive.path, verbose=verbose)
                    if stats.unchanged:
                        already_imported += 1
                        logger.info("%s: source checksum unchanged", item_label)
                        continue
                    parsing_seconds += stats.parsing_seconds
                    database_seconds += stats.database_seconds
                    imported += 1
                    if not verbose:
                        logger.info("%s: imported %s data rows", item_label, f"{stats.rows:,}")
                except DatasetUnavailable as exc:
                    unavailable += 1
                    logger.warning("%s: unavailable (%s)", item_label, exc)
                except Exception:
                    logger.exception("%s: stopped by unexpected error", item_label)
                    raise
    finally:
        for future in pending:
            future.cancel()
        executor.shutdown(wait=True, cancel_futures=True)

    logger.info(
        f"{summary_prefix}: imported={imported}, already_imported={already_imported}, unavailable={unavailable}, "
        f"elapsed={format_duration(time.monotonic() - started_at)}, "
        f"download_worker_time={format_duration(download_seconds)}, "
        f"csv_parsing={format_duration(parsing_seconds)}, database={format_duration(database_seconds)}"
    )
    return RefreshSummary(imported, already_imported, unavailable)


def refresh_all_sales(
    connection,
    output_dir: Path,
    start_year: int,
    end_year: int,
    force: bool,
    workers: int,
    verbose: bool = False,
) -> RefreshSummary:
    sync_reference_data(connection)
    with connection.cursor() as cursor:
        cursor.execute("SELECT code FROM municipalities ORDER BY code")
        municipalities = [row[0] for row in cursor]
        cursor.execute("SELECT municipality_code,dataset_year FROM source_files WHERE municipality_code IS NOT NULL")
        imported = set(cursor.fetchall())
    connection.commit()
    all_items = [(code, year) for year in range(start_year, end_year + 1) for code in municipalities]
    items = [item for item in all_items if force or item not in imported]
    return _run_refresh(
        connection,
        items,
        skipped=len(all_items) - len(items),
        workers=workers,
        verbose=verbose,
        thread_name="jgp-download",
        prepare=lambda item: prepare_archive(*item, output_dir, force),
        label=lambda position, total, item: (
            f"[{position}/{total}] municipality={item[0]} year={item[1]}"
        ),
        summary_prefix="Finished municipality import",
    )


def refresh_all_national(
    connection,
    output_dir: Path,
    end_year: int,
    force: bool,
    workers: int,
    verbose: bool = False,
    refresh_current_year: bool = False,
) -> RefreshSummary:
    sync_reference_data(connection)
    with connection.cursor() as cursor:
        cursor.execute(
            """SELECT dataset_type,dataset_year FROM source_files
               WHERE dataset_scope='national' AND dataset_type IN ('sales','rent')"""
        )
        imported = set(cursor.fetchall())
    connection.commit()
    all_items = [
        (dataset_type, year)
        for dataset_type, product in NATIONAL_DATASETS.items()
        for year in range(product.start_year, end_year + 1)
    ]
    items = [
        item
        for item in all_items
        if force
        or item not in imported
        or (refresh_current_year and item[1] == end_year)
    ]
    return _run_refresh(
        connection,
        items,
        skipped=len(all_items) - len(items),
        workers=workers,
        verbose=verbose,
        thread_name="jgp-national-download",
        prepare=lambda item: prepare_national_archive(
            *item,
            output_dir,
            force or (refresh_current_year and item[1] == end_year),
        ),
        label=lambda position, total, item: (
            f"[{position}/{total}] national {item[0]} year={item[1]}"
        ),
        summary_prefix="Finished national import",
    )
