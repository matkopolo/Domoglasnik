"""Typed values shared across importer and pipeline modules."""

from dataclasses import dataclass
from pathlib import Path


@dataclass
class ImportStats:
    parsing_seconds: float = 0.0
    database_seconds: float = 0.0
    rows: int = 0
    codebook_rows: int = 0
    codebook_skipped: bool = False


@dataclass
class ArchiveResult:
    path: Path
    reused: bool
    download_seconds: float
