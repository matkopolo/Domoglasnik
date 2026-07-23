"""Stable configuration and dataset metadata."""

from __future__ import annotations

import os
from dataclasses import dataclass

API_BASE = "https://ipi.eprostor.gov.si/jgp-service-api"
RPE_OGC_BASE = "https://ipi.eprostor.gov.si/wfs-si-gurs-rpe/ogc/features"
DEFAULT_DATABASE_URL = "postgresql://real_estate:real_estate@localhost:5432/real_estate"
AVAILABLE_THREADS = os.cpu_count() or 1
DEFAULT_WORKERS = min(6, AVAILABLE_THREADS)
MAX_DOWNLOAD_ATTEMPTS = 5


@dataclass(frozen=True)
class NationalDataset:
    product_id: int
    archive_code: str
    start_year: int


NATIONAL_DATASETS = {
    "sales": NationalDataset(product_id=321, archive_code="KPP", start_year=2007),
    "rent": NationalDataset(product_id=322, archive_code="NP", start_year=2013),
}
