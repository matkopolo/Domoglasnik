"""Compatibility imports for callers using the original application module.

New code should import archive conversion from :mod:`importer`, orchestration
from :mod:`pipeline`, and the command entry point from :mod:`cli`.
"""

from .cli import main
from .importer import RENT_TABLES, TABLES, converted, import_zip
from .models import ArchiveResult, ImportStats
from .pipeline import (
    fetch_national_zip,
    fetch_zip,
    format_duration,
    prepare_archive,
    prepare_national_archive,
    refresh_all_national,
    refresh_all_sales,
    sync_reference_data,
    validate_worker_count,
)

__all__ = [
    "RENT_TABLES",
    "TABLES",
    "ArchiveResult",
    "ImportStats",
    "converted",
    "fetch_national_zip",
    "fetch_zip",
    "format_duration",
    "import_zip",
    "main",
    "prepare_archive",
    "prepare_national_archive",
    "refresh_all_national",
    "refresh_all_sales",
    "sync_reference_data",
    "validate_worker_count",
]
