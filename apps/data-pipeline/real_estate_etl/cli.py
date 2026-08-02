"""Command-line interface for the Slovenian real-estate data pipeline."""

from __future__ import annotations

import argparse
import os
import sys
from datetime import date
from pathlib import Path

import psycopg

from .geography import sync_geographies
from .importer import import_zip
from .operations import (
    DEFAULT_INTERVAL_DAYS,
    DEFAULT_RETRY_HOURS,
    refresh_analytics,
    refresh_system,
    reset_and_repopulate,
    scheduled_refresh_loop,
)
from .pipeline import (
    fetch_national_zip,
    fetch_zip,
    refresh_all_national,
    refresh_all_sales,
    sync_reference_data,
    validate_worker_count,
)
from .settings import AVAILABLE_THREADS, DEFAULT_DATABASE_URL, DEFAULT_WORKERS, NATIONAL_DATASETS


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Fetch and import Slovenian JGP real-estate data")
    parser.add_argument("--database-url", default=os.getenv("DATABASE_URL", DEFAULT_DATABASE_URL))
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("sync-reference-data")
    commands.add_parser("sync-geographies", help="Import current GURS RPE boundaries into the analytics schema")

    models = commands.add_parser(
        "refresh-models",
        help="Cross-fit hedonic and Isolation Forest models and refresh anomaly scores",
    )
    _add_workers(models, "Isolation Forest workers")

    fetch = commands.add_parser("fetch")
    fetch.add_argument("--municipality", type=int, required=True)
    fetch.add_argument("--year", type=int, required=True)
    fetch.add_argument("--output-dir", type=Path, default=Path("data/raw"))

    fetch_national = commands.add_parser("fetch-national", help="Download one national BUY or RENT yearly archive")
    fetch_national.add_argument("--type", choices=tuple(NATIONAL_DATASETS), required=True, dest="dataset_type")
    fetch_national.add_argument("--year", type=int, required=True)
    fetch_national.add_argument("--output-dir", type=Path, default=Path("data/raw"))

    load = commands.add_parser("import")
    load.add_argument("zip_file", type=Path)
    load.add_argument("--verbose", action="store_true")

    refresh = commands.add_parser("refresh")
    refresh.add_argument("--municipality", type=int, required=True)
    refresh.add_argument("--year", type=int, required=True)
    refresh.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    refresh.add_argument("--verbose", action="store_true")

    all_sales = commands.add_parser("refresh-all-sales", help="Download and import all municipality/year sales data")
    all_sales.add_argument("--start-year", type=int, default=2007)
    all_sales.add_argument("--end-year", type=int, default=date.today().year)
    _add_refresh_options(all_sales, "Refresh combinations already present in source_files")

    all_national = commands.add_parser(
        "refresh-all-data",
        help="Download and import all national yearly BUY and RENT archives",
    )
    all_national.add_argument("--end-year", type=int, default=date.today().year)
    _add_refresh_options(all_national, "Refresh national years already imported")

    system = commands.add_parser(
        "refresh-system",
        help="Incrementally import GURS data, synchronize geography, and refresh analytics/models",
    )
    system.add_argument("--end-year", type=int, default=date.today().year)
    _add_refresh_options(system, "Download current source versions even when already imported")

    commands.add_parser(
        "refresh-analytics",
        help="Refresh deterministic analytical facts, outliers, quality profiles, and indices",
    )

    reset = commands.add_parser(
        "reset-and-repopulate",
        help="Delete imported/derived market data and rebuild it; preserves users/preferences",
    )
    reset.add_argument("--confirm-reset", action="store_true")
    reset.add_argument("--end-year", type=int, default=date.today().year)
    reset.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    _add_workers(reset, "Parallel download and model workers")
    reset.add_argument("--verbose", action="store_true")

    schedule = commands.add_parser(
        "schedule",
        help="Run a complete refresh immediately and then every configured interval",
    )
    schedule.add_argument(
        "--interval-days",
        type=int,
        default=int(os.getenv("REFRESH_INTERVAL_DAYS", DEFAULT_INTERVAL_DAYS)),
    )
    schedule.add_argument(
        "--retry-hours",
        type=int,
        default=int(os.getenv("REFRESH_RETRY_HOURS", DEFAULT_RETRY_HOURS)),
    )
    schedule.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    schedule.add_argument("--health-file", type=Path)
    _add_workers(schedule, "Parallel download and model workers")
    schedule.add_argument("--verbose", action="store_true")
    return parser


def _add_workers(parser: argparse.ArgumentParser, description: str) -> None:
    parser.add_argument(
        "--workers",
        type=int,
        default=DEFAULT_WORKERS,
        metavar="N",
        help=f"{description} (default: {DEFAULT_WORKERS}; machine limit: {AVAILABLE_THREADS})",
    )


def _add_refresh_options(parser: argparse.ArgumentParser, force_help: str) -> None:
    parser.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    parser.add_argument("--force", action="store_true", help=force_help)
    _add_workers(parser, "Parallel download workers")
    parser.add_argument("--verbose", action="store_true", help="Print per-table row counts and codebook details")


def validate_arguments(parser: argparse.ArgumentParser, args: argparse.Namespace) -> None:
    if args.command == "fetch-national":
        start_year = NATIONAL_DATASETS[args.dataset_type].start_year
        if args.year < start_year:
            parser.error(f"{args.dataset_type} national data starts in {start_year}")
    if args.command == "reset-and-repopulate" and not args.confirm_reset:
        parser.error("reset-and-repopulate requires --confirm-reset")
    if args.command == "schedule":
        if args.interval_days < 1:
            parser.error("--interval-days must be at least 1")
        if args.retry_hours < 1:
            parser.error("--retry-hours must be at least 1")
    if args.command not in {
        "refresh-all-sales",
        "refresh-all-data",
        "refresh-models",
        "refresh-system",
        "reset-and-repopulate",
        "schedule",
    }:
        return
    if args.command == "refresh-all-sales" and args.start_year > args.end_year:
        parser.error("--start-year must not be later than --end-year")
    if args.command in {"refresh-all-data", "refresh-system", "reset-and-repopulate"} and (
        args.end_year < NATIONAL_DATASETS["sales"].start_year
    ):
        parser.error("--end-year must be 2007 or later")
    try:
        validate_worker_count(args.workers)
    except ValueError as exc:
        parser.error(str(exc))


def run(args: argparse.Namespace) -> None:
    if args.command == "fetch":
        fetch_zip(args.municipality, args.year, args.output_dir)
        return
    if args.command == "fetch-national":
        fetch_national_zip(args.dataset_type, args.year, args.output_dir)
        return
    if args.command == "schedule":
        scheduled_refresh_loop(
            args.database_url,
            output_dir=args.output_dir,
            interval_days=args.interval_days,
            retry_hours=args.retry_hours,
            workers=args.workers,
            verbose=args.verbose,
            health_file=args.health_file,
        )
        return

    with psycopg.connect(args.database_url) as connection:
        if args.command == "sync-reference-data":
            sync_reference_data(connection)
        elif args.command == "sync-geographies":
            sync_geographies(connection)
        elif args.command == "refresh-models":
            from .model_reporting import format_model_report
            from .modeling import refresh_models

            for result in refresh_models(connection, args.workers):
                print(format_model_report(result))
        elif args.command == "import":
            import_zip(connection, args.zip_file, verbose=args.verbose)
        elif args.command == "refresh":
            sync_reference_data(connection)
            archive = fetch_zip(args.municipality, args.year, args.output_dir)
            import_zip(connection, archive, verbose=args.verbose)
        elif args.command == "refresh-all-sales":
            refresh_all_sales(
                connection,
                args.output_dir,
                args.start_year,
                args.end_year,
                args.force,
                args.workers,
                args.verbose,
            )
        elif args.command == "refresh-all-data":
            refresh_all_national(
                connection,
                args.output_dir,
                args.end_year,
                args.force,
                args.workers,
                args.verbose,
            )
        elif args.command == "refresh-system":
            refresh_system(
                connection,
                output_dir=args.output_dir,
                end_year=args.end_year,
                force=args.force,
                workers=args.workers,
                verbose=args.verbose,
            )
        elif args.command == "refresh-analytics":
            counts = refresh_analytics(connection)
            print(
                "Refreshed analytics: "
                + ", ".join(
                    f"{name.rsplit('.', 1)[-1]}={count}"
                    for name, count in counts.items()
                )
            )
        elif args.command == "reset-and-repopulate":
            reset_and_repopulate(
                connection,
                output_dir=args.output_dir,
                end_year=args.end_year,
                workers=args.workers,
                verbose=args.verbose,
            )


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()
    validate_arguments(parser, args)
    run(args)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1) from None
