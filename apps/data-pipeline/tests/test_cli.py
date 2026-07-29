import zipfile
from datetime import date
from decimal import Decimal
from pathlib import Path
from unittest.mock import Mock, patch

import httpx
import pytest

from real_estate_etl.api import DatasetUnavailable, JGPClient
from real_estate_etl.importer import converted
from real_estate_etl.models import ArchiveResult
from real_estate_etl.pipeline import (
    format_duration,
    prepare_archive,
    prepare_national_archive,
    validate_worker_count,
)


@pytest.mark.parametrize(
    ("field", "raw", "expected"),
    [
        ("transaction_id", " -42 ", -42),
        ("vat_included", "1", True),
        ("vat_included", "0", False),
        ("contract_date", "09.01.2026", date(2026, 1, 9)),
        ("rental_start_date", "00.00.0000", None),
        ("contract_price", "12,50", Decimal("12.50")),
        ("parcel_number", " 10/2 ", "10/2"),
        ("cadastral_municipality_code", "", None),
    ],
)
def test_converted(field, raw, expected):
    assert converted(field, raw) == expected


@pytest.mark.parametrize(
    ("seconds", "expected"),
    [(42.9, "42s"), (125, "2m 5s"), (7384, "2h 3m 4s")],
)
def test_format_duration(seconds, expected):
    assert format_duration(seconds) == expected


def test_worker_count_is_bounded_by_machine_threads():
    validate_worker_count(1, available_threads=4)
    validate_worker_count(4, available_threads=4)
    with pytest.raises(ValueError, match="at least 1"):
        validate_worker_count(0, available_threads=4)
    with pytest.raises(ValueError, match="cannot exceed"):
        validate_worker_count(5, available_threads=4)


def test_prepare_archive_reuses_valid_zip(tmp_path: Path):
    archive = tmp_path / "ETN_001_2026_KPP_test.zip"
    with zipfile.ZipFile(archive, "w") as output:
        output.writestr("test.csv", "a\n1\n")

    with patch("real_estate_etl.pipeline.fetch_zip") as fetch:
        result = prepare_archive(1, 2026, tmp_path, force=False)

    assert isinstance(result, ArchiveResult)
    assert result.path == archive
    assert result.reused is True
    fetch.assert_not_called()


def test_prepare_archive_force_downloads(tmp_path: Path):
    downloaded = tmp_path / "new.zip"
    with patch("real_estate_etl.pipeline.fetch_zip", return_value=downloaded) as fetch:
        result = prepare_archive(1, 2026, tmp_path, force=True)

    assert result.path == downloaded
    assert result.reused is False
    fetch.assert_called_once_with(1, 2026, tmp_path)


def test_prepare_national_archive_reuses_valid_zip(tmp_path: Path):
    archive = tmp_path / "ETN_SLO_2020_KPP_test.zip"
    with zipfile.ZipFile(archive, "w") as output:
        output.writestr("test.csv", "a\n1\n")
    with patch("real_estate_etl.pipeline.fetch_national_zip") as fetch:
        result = prepare_national_archive("sales", 2020, tmp_path, force=False)
    assert result.path == archive
    assert result.reused is True
    fetch.assert_not_called()


def test_fetch_retries_transient_connection_errors(tmp_path: Path):
    downloaded = tmp_path / "result.zip"
    request = httpx.Request("GET", "https://example.test/data")
    transient = httpx.ReadError("connection reset", request=request)
    sleep = Mock()
    client = JGPClient(tmp_path, sleep=sleep)
    with (
        patch.object(client, "_fetch_municipality_once", side_effect=[transient, transient, downloaded]) as fetch_once,
    ):
        result = client.fetch_municipality(3, 2023)

    assert result == downloaded
    assert fetch_once.call_count == 3
    assert [call.args[0] for call in sleep.call_args_list] == [1, 2]


def test_fetch_does_not_retry_unavailable_dataset(tmp_path: Path):
    sleep = Mock()
    client = JGPClient(tmp_path, sleep=sleep)
    with (
        patch.object(client, "_fetch_municipality_once", side_effect=DatasetUnavailable("missing")) as fetch_once,
        pytest.raises(DatasetUnavailable),
    ):
        client.fetch_municipality(3, 2023)

    fetch_once.assert_called_once()
    sleep.assert_not_called()
