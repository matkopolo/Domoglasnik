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
from .pipeline import (
    fetch_national_zip,
    fetch_zip,
    format_duration,
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
    if args.command not in {"refresh-all-sales", "refresh-all-data", "refresh-models"}:
        return
    if args.command == "refresh-all-sales" and args.start_year > args.end_year:
        parser.error("--start-year must not be later than --end-year")
    if args.command == "refresh-all-data" and args.end_year < NATIONAL_DATASETS["sales"].start_year:
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

    with psycopg.connect(args.database_url) as connection:
        if args.command == "sync-reference-data":
            sync_reference_data(connection)
        elif args.command == "sync-geographies":
            sync_geographies(connection)
        elif args.command == "refresh-models":
            from .modeling import refresh_models

            for result in refresh_models(connection, args.workers):
                print(
                    f"Refreshed {result.transaction_kind} model: rows={result.rows}, "
                    f"version={result.model_version}, elapsed={format_duration(result.elapsed_seconds)}, "
                    f"metrics={result.metrics}"
                )
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
