"""Thread-safe client for downloading public GURS JGP datasets."""

from __future__ import annotations

import sys
import threading
import time
import urllib.parse
import zipfile
from pathlib import Path

import httpx
from tenacity import Retrying, retry_if_exception, stop_after_attempt, wait_exponential

from .settings import API_BASE, MAX_DOWNLOAD_ATTEMPTS, NATIONAL_DATASETS

_CLIENTS_LOCAL = threading.local()


class DatasetUnavailable(Exception):
    """The API has no prepared dataset for the requested filters."""


class JGPClient:
    """Reusable HTTP client with per-thread connection pools and retries."""

    def __init__(self, output_dir: Path, max_attempts: int = MAX_DOWNLOAD_ATTEMPTS, sleep=time.sleep):
        self.output_dir = output_dir
        self.max_attempts = max_attempts
        self._sleep = sleep
        self._local = threading.local()

    def _http(self) -> httpx.Client:
        client = getattr(self._local, "client", None)
        if client is None:
            client = httpx.Client(
                timeout=httpx.Timeout(180, connect=30),
                follow_redirects=True,
                headers={"User-Agent": "real-estate-etl/1.0"},
            )
            self._local.client = client
        return client

    def get_json(self, url: str):
        response = self._http().get(url)
        response.raise_for_status()
        return response.json()

    def get_json_retried(self, url: str, label: str = "API request"):
        return self._with_retries(lambda: self.get_json(url), label)

    def _download(self, url: str, target: Path) -> Path:
        self.output_dir.mkdir(parents=True, exist_ok=True)
        temporary = target.with_suffix(target.suffix + ".part")
        try:
            with self._http().stream("GET", url) as response, temporary.open("wb") as output:
                response.raise_for_status()
                for chunk in response.iter_bytes(1024 * 1024):
                    output.write(chunk)
            if not zipfile.is_zipfile(temporary):
                raise ValueError(f"Downloaded file is not a valid ZIP: {temporary}")
            temporary.replace(target)
        finally:
            if temporary.exists():
                temporary.unlink()
        return target

    @staticmethod
    def _retryable(exc: Exception) -> bool:
        if isinstance(exc, httpx.TransportError):
            return True
        if isinstance(exc, httpx.HTTPStatusError):
            status = exc.response.status_code
            return status in {408, 425, 429} or status >= 500
        return False

    def _with_retries(self, operation, label: str):
        def before_sleep(state):
            print(
                f"Transient {label} error; retry {state.attempt_number}/{self.max_attempts - 1} "
                f"in {state.next_action.sleep:g}s: {state.outcome.exception()}",
                file=sys.stderr,
            )

        retryer = Retrying(
            stop=stop_after_attempt(self.max_attempts),
            wait=wait_exponential(multiplier=1, min=1, max=16),
            retry=retry_if_exception(self._retryable),
            before_sleep=before_sleep,
            reraise=True,
            sleep=self._sleep,
        )
        return retryer(operation)

    def fetch_municipality(self, municipality: int, year: int) -> Path:
        return self._with_retries(
            lambda: self._fetch_municipality_once(municipality, year),
            f"municipality={municipality} year={year} download",
        )

    def _fetch_municipality_once(self, municipality: int, year: int) -> Path:
        query = urllib.parse.urlencode({"filterParam": "OBCINE", "filterValue": municipality, "filterYear": year})
        base = f"{API_BASE}/display-views/groups/131/composite-products/323"
        result = self.get_json(f"{base}/result?{query}")
        if result.get("statusText") != "OK" or not result.get("file"):
            raise DatasetUnavailable(f"Dataset is not ready: {result}")
        download = self.get_json(f"{base}/file?{query}")
        target = self.output_dir / result["file"]["name"]
        path = self._download(download["url"], target)
        print(f"Downloaded {path}")
        return path

    def fetch_national(self, dataset_type: str, year: int) -> Path:
        return self._with_retries(
            lambda: self._fetch_national_once(dataset_type, year),
            f"national {dataset_type} year={year} download",
        )

    def _fetch_national_once(self, dataset_type: str, year: int) -> Path:
        product = NATIONAL_DATASETS[dataset_type]
        query = urllib.parse.urlencode({"filterParam": "DRZAVA", "filterValue": 1, "filterYear": year})
        base = f"{API_BASE}/display-views/groups/127/composite-products/{product.product_id}"
        result = self.get_json(f"{base}/result?{query}")
        if result.get("statusText") != "OK" or not result.get("file"):
            raise DatasetUnavailable(f"National {dataset_type} dataset is not ready: {result}")
        download = self.get_json(f"{base}/file?{query}")
        target = self.output_dir / result["file"]["name"]
        path = self._download(download["url"], target)
        print(f"Downloaded national {dataset_type}: {path}")
        return path


def client_for(output_dir: Path) -> JGPClient:
    """Return a reusable client scoped to the current worker thread and output directory."""
    clients = getattr(_CLIENTS_LOCAL, "clients", None)
    if clients is None:
        clients = {}
        _CLIENTS_LOCAL.clients = clients
    key = str(output_dir.resolve())
    if key not in clients:
        clients[key] = JGPClient(output_dir)
    return clients[key]
