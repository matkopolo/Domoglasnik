from __future__ import annotations

import argparse
import csv
import hashlib
import io
import os
import re
import sys
import threading
import time
import urllib.parse
import zipfile
from concurrent.futures import FIRST_COMPLETED, Future, ThreadPoolExecutor, wait
from dataclasses import dataclass
from datetime import date
from datetime import datetime, timezone
from pathlib import Path

import httpx
import psycopg
from psycopg import sql

API_BASE = "https://ipi.eprostor.gov.si/jgp-service-api"
DEFAULT_DATABASE_URL = "postgresql://real_estate:real_estate@localhost:5432/real_estate"
AVAILABLE_THREADS = os.cpu_count() or 1
DEFAULT_WORKERS = min(6, AVAILABLE_THREADS)
MAX_DOWNLOAD_ATTEMPTS = 5
_HTTP_LOCAL = threading.local()
NATIONAL_PRODUCTS = {
    "sales": {"product_id": 321, "archive_code": "KPP", "start_year": 2007},
    "rent": {"product_id": 322, "archive_code": "NP", "start_year": 2013},
}


class DatasetUnavailable(Exception):
    """The API has no prepared dataset for a municipality/year combination."""


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

TABLES = {
    "POSLI": ("transactions", {
        "ID_POSLA": "transaction_id", "VRSTA_KUPOPRODAJNEGA_POSLA": "transaction_type",
        "DATUM_UVELJAVITVE": "effective_date", "DATUM_SKLENITVE_POGODBE": "contract_date",
        "POGODBENA_CENA_ODSKODNINA": "contract_price", "VKLJUCENOST_DDV": "vat_included",
        "STOPNJA_DDV": "vat_rate", "DATUM_IZTEKA_LIZINGA": "lease_expiry_date",
        "DATUM_PRENEHANJA_LIZINGA": "lease_termination_date", "OPOMBE_O_PRAVNEM_POSLU": "notes",
        "POSREDOVANJE_NEPREMICNINSKE_AGENCIJE": "agency_brokered",
        "DATUM_ZADNJE_SPREMEMBE_POSLA": "last_changed_date",
        "DATUM_ZADNJE_UVELJAVITVE_POSLA": "last_effective_date", "VRSTA_AKTA": "deed_type",
        "TRZNOST_POSLA": "marketability", "LETO": "dataset_year", "RPE_OBCINE_SIFRA": "municipality_code",
    }, ["transaction_id"]),
    "DELISTAVB": ("building_parts", {
        "ID_POSLA": "transaction_id", "SIFRA_KO": "cadastral_municipality_code",
        "IME_KO": "cadastral_municipality_name", "OBCINA": "municipality_name",
        "STEVILKA_STAVBE": "building_number", "STEVILKA_DELA_STAVBE": "building_part_number",
        "PARCELNA_STEVILKA_ZA_GEOLOKACIJO": "parcel_number_for_geolocation",
        "INTERNA_OZNAKA_DELA_STAVBE": "internal_building_part_label",
        "EVIDENTIRANOST_DELA_STAVBE": "recorded_status", "NASELJE": "settlement", "ULICA": "street",
        "HISNA_STEVILKA": "house_number", "DODATEK_HS": "house_number_suffix",
        "STEVILKA_STANOVANJA_ALI_POSLOVNEGA_PROSTORA": "apartment_or_business_unit_number",
        "VRSTA_DELA_STAVBE": "building_part_type", "LETO_IZGRADNJE_DELA_STAVBE": "construction_year",
        "STAVBA_JE_DOKONCANA": "building_completed", "GRADBENA_FAZA": "construction_phase",
        "NOVOGRADNJA": "new_construction", "PRODANA_POVRSINA": "sold_area",
        "PRODANI_DELEZ_DELA_STAVBE": "sold_share", "PRODANA_POVRSINA_DELA_STAVBE": "sold_building_part_area",
        "PRODANA_UPORABNA_POVRSINA_DELA_STAVBE": "sold_usable_area", "NADSTROPJE_DELA_STAVBE": "floor",
        "STEVILO_ZUNANJIH_PARKIRNIH_MEST": "outdoor_parking_spaces", "ATRIJ": "atrium",
        "POVRSINA_ATRIJA": "atrium_area", "OPOMBE_O_NEPREMICNINI": "property_notes",
        "DEJANSKA_RABA_DELA_STAVBE": "actual_use", "LEGA_DELA_STAVBE_V_STAVBI": "position_in_building",
        "STEVILO_SOB": "room_count", "POVRSINA_DELA_STAVBE": "building_part_area",
        "UPORABNA_POVRSINA": "usable_area", "PROSTORI_DELA_STAVBE": "rooms",
        "POGODBENA_CENA_DELA_STAVBE": "contractual_price", "STOPNJA_DDV_DELA_STAVBE": "vat_rate",
        "E_CENTROID": "centroid_e", "N_CENTROID": "centroid_n", "LETO": "dataset_year",
        "RPE_OBCINE_SIFRA": "municipality_code",
    }, ["transaction_id", "cadastral_municipality_code", "building_number", "building_part_number"]),
    "ZEMLJISCA": ("land_parcels", {
        "ID_POSLA": "transaction_id", "SIFRA_KO": "cadastral_municipality_code",
        "IME_KO": "cadastral_municipality_name", "OBCINA": "municipality_name",
        "PARCELNA_STEVILKA": "parcel_number", "VRSTA_ZEMLJISCA": "land_type",
        "VRSTA_TRAJNEGA_NASADA": "permanent_crop_type", "STAROST_TRAJNEGA_NASADA": "permanent_crop_age",
        "PRODANI_DELEZ_PARCELE": "sold_share", "OPOMBE_O_NEPREMICNINI": "property_notes",
        "POVRSINA_PARCELE": "parcel_area", "POGODBENA_CENA_PARCELE": "contractual_price",
        "STOPNJA_DDV_PARCELE": "vat_rate", "E_CENTROID": "centroid_e", "N_CENTROID": "centroid_n",
        "LETO": "dataset_year", "RPE_OBCINE_SIFRA": "municipality_code",
    }, ["transaction_id", "cadastral_municipality_code", "parcel_number"]),
}

RENT_TABLES = {
    "POSLI": ("rental_transactions", {
        "ID_POSLA": "transaction_id", "VRSTA_NAJEMNEGA_POSLA": "rental_transaction_type",
        "DATUM_UVELJAVITVE": "effective_date", "DATUM_SKLENITVE_POGODBE": "contract_date",
        "POGODBENA_NAJEMNINA": "contract_rent",
        "VKLJUCENOST_OBRATOVALNIH_STROSKOV_V_NAJEMNINO": "operating_costs_included",
        "VKLJUCENOST_DDV": "vat_included", "STOPNJA_DDV": "vat_rate",
        "DATUM_ZACETKA_NAJEMA": "rental_start_date", "DATUM_PRENEHANJA_NAJEMA": "rental_end_date",
        "CAS_NAJEMA": "rental_time", "TRAJANJE_NAJEMA": "rental_duration",
        "DATUM_ZAKLJUCKA_NAJEMA_DATUM_PREDCASNE_PREKINITVE_NAJEMA": "closure_or_early_termination_date",
        "OPOMBE_O_PRAVNEM_POSLU": "notes",
        "POSREDOVANJE_NEPREMICNINSKE_AGENCIJE": "agency_brokered",
        "DATUM_ZADNJE_SPREMEMBE_POSLA": "last_changed_date",
        "DATUM_ZADNJE_UVELJAVITVE_POSLA": "last_effective_date", "VRSTA_AKTA": "deed_type",
        "TRZNOST_POSLA": "marketability", "LETO": "dataset_year",
    }, ["transaction_id"]),
    "DELISTAVB": ("rental_building_parts", {
        "ID_POSLA": "transaction_id", "SIFRA_KO": "cadastral_municipality_code",
        "IME_KO": "cadastral_municipality_name", "OBCINA": "municipality_name",
        "STEVILKA_STAVBE": "building_number", "STEVILKA_DELA_STAVBE": "building_part_number",
        "INTERNA_OZNAKA_ODDANIH_PROSTOROV": "internal_rented_spaces_label", "NASELJE": "settlement",
        "ULICA": "street", "HISNA_STEVILKA": "house_number", "DODATEK_HS": "house_number_suffix",
        "STEVILKA_STANOVANJA_ALI_POSLOVNEGA_PROSTORA": "apartment_or_business_unit_number",
        "VRSTA_ODDANIH_PROSTOROV": "rented_spaces_type", "OPREMLJENOST_ODDANIH_PROSTOROV": "furnishing",
        "MIKROLOKACIJA_ODDANIH_PROSTOROV": "microlocation", "IZLOZBA": "storefront",
        "NAKUPOVALNI_CENTER": "shopping_center", "POVRSINA_ODDANIH_PROSTOROV": "rented_spaces_area",
        "POVRSINA_ODDANIH_PROSTOROV_ENAKA_DELU_STAVBE": "rented_area_equals_building_part",
        "POGODBENA_NAJEMNINA_POSAMEZNIH_ODDANIH_PROSTOROV": "contractual_rent",
        "OPOMBE_O_ODDANIH_PROSTORIH": "rented_spaces_notes",
        "LETO_IZGRADNJE_STAVBE": "construction_year", "DEJANSKA_RABA_DELA_STAVBE": "actual_use",
        "LEGA_DELA_STAVBE_V_STAVBI": "position_in_building", "STEVILO_SOB": "room_count",
        "POVRSINA_DELA_STAVBE": "building_part_area",
        "UPORABNA_POVRSINA_DELA_STAVBE": "usable_building_part_area", "PROSTORI_DELA_STAVBE": "rooms",
        "UPORABNA_POVRSINA_ODDANIH_PROSTOROV": "rented_usable_area", "GOSTINSKI_VRT": "hospitality_garden",
        "VKLJUCENOST_GOSTINSKEGA_VRTA_V_NAJEMNINO": "garden_included_in_rent",
        "POVRSINA_GOSTINSKEGA_VRTA": "hospitality_garden_area", "E_CENTROID": "centroid_e",
        "N_CENTROID": "centroid_n", "LETO": "dataset_year",
    }, ["transaction_id"]),
}

INT_FIELDS = {"transaction_id", "transaction_type", "deed_type", "marketability", "dataset_year",
              "municipality_code", "cadastral_municipality_code", "recorded_status", "building_part_type",
              "construction_year", "construction_phase", "outdoor_parking_spaces", "land_type",
              "permanent_crop_type", "permanent_crop_age", "rental_transaction_type",
              "rented_spaces_type", "furnishing", "microlocation"}
BOOL_FIELDS = {"vat_included", "agency_brokered", "building_completed", "new_construction", "atrium"}
BOOL_FIELDS.update({"operating_costs_included", "storefront", "shopping_center",
                    "rented_area_equals_building_part", "hospitality_garden", "garden_included_in_rent"})
DATE_FIELDS = {"effective_date", "contract_date", "lease_expiry_date", "lease_termination_date",
               "last_changed_date", "last_effective_date", "rental_start_date", "rental_end_date",
               "closure_or_early_termination_date"}
NULL_DATE_SENTINELS = {"00.00.0000", "0.0.0000", "00.00.00"}
NUMERIC_FIELDS = {"contract_price", "vat_rate", "sold_area", "sold_building_part_area", "sold_usable_area",
                  "atrium_area", "room_count", "building_part_area", "usable_area", "contractual_price",
                  "centroid_e", "centroid_n", "parcel_area", "contract_rent", "rented_spaces_area",
                  "usable_building_part_area", "rented_usable_area", "hospitality_garden_area"}


def http_client() -> httpx.Client:
    client = getattr(_HTTP_LOCAL, "client", None)
    if client is None:
        client = httpx.Client(
            timeout=httpx.Timeout(180, connect=30),
            follow_redirects=True,
            headers={"User-Agent": "real-estate-etl/1.0"},
        )
        _HTTP_LOCAL.client = client
    return client


def get_json(url: str):
    response = http_client().get(url)
    response.raise_for_status()
    return response.json()


def sync_reference_data(connection) -> None:
    municipalities = get_json(f"{API_BASE}/municipalities")
    cadastral = get_json(f"{API_BASE}/register-municipalities")
    with connection.cursor() as cur:
        cur.executemany("INSERT INTO municipalities(code,name) VALUES (%s,%s) ON CONFLICT(code) DO UPDATE SET name=excluded.name",
                        [(int(x["sifra"]), x["name"]) for x in municipalities])
        cur.executemany("INSERT INTO cadastral_municipalities(code,name) VALUES (%s,%s) ON CONFLICT(code) DO UPDATE SET name=excluded.name",
                        [(int(x["sifko"]), x["name"]) for x in cadastral])
    connection.commit()
    print(f"Synced {len(municipalities)} municipalities and {len(cadastral)} cadastral municipalities")


def fetch_zip_once(municipality: int, year: int, output_dir: Path) -> Path:
    query = urllib.parse.urlencode({"filterParam": "OBCINE", "filterValue": municipality, "filterYear": year})
    result = get_json(f"{API_BASE}/display-views/groups/131/composite-products/323/result?{query}")
    if result.get("statusText") != "OK" or not result.get("file"):
        raise DatasetUnavailable(f"Dataset is not ready: {result}")
    download = get_json(f"{API_BASE}/display-views/groups/131/composite-products/323/file?{query}")
    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / result["file"]["name"]
    temporary = target.with_suffix(target.suffix + ".part")
    try:
        with http_client().stream("GET", download["url"]) as response, temporary.open("wb") as out:
            response.raise_for_status()
            for chunk in response.iter_bytes(1024 * 1024):
                out.write(chunk)
        if not zipfile.is_zipfile(temporary):
            raise ValueError(f"Downloaded file is not a valid ZIP: {temporary}")
        temporary.replace(target)
    finally:
        if temporary.exists():
            temporary.unlink()
    print(f"Downloaded {target}")
    return target


def retryable_http_error(exc: Exception) -> bool:
    if isinstance(exc, httpx.TransportError):
        return True
    if isinstance(exc, httpx.HTTPStatusError):
        return exc.response.status_code in {408, 425, 429} or exc.response.status_code >= 500
    return False


def fetch_zip(municipality: int, year: int, output_dir: Path) -> Path:
    for attempt in range(1, MAX_DOWNLOAD_ATTEMPTS + 1):
        try:
            return fetch_zip_once(municipality, year, output_dir)
        except DatasetUnavailable:
            raise
        except Exception as exc:
            if not retryable_http_error(exc) or attempt == MAX_DOWNLOAD_ATTEMPTS:
                raise
            delay = min(2 ** (attempt - 1), 16)
            print(
                f"Transient download error for municipality={municipality} year={year}; "
                f"retry {attempt}/{MAX_DOWNLOAD_ATTEMPTS - 1} in {delay}s: {exc}",
                file=sys.stderr,
            )
            time.sleep(delay)
    raise AssertionError("unreachable")


def fetch_national_zip_once(dataset_type: str, year: int, output_dir: Path) -> Path:
    product = NATIONAL_PRODUCTS[dataset_type]
    query = urllib.parse.urlencode({"filterParam": "DRZAVA", "filterValue": 1, "filterYear": year})
    base = f"{API_BASE}/display-views/groups/127/composite-products/{product['product_id']}"
    result = get_json(f"{base}/result?{query}")
    if result.get("statusText") != "OK" or not result.get("file"):
        raise DatasetUnavailable(f"National {dataset_type} dataset is not ready: {result}")
    download = get_json(f"{base}/file?{query}")
    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / result["file"]["name"]
    temporary = target.with_suffix(target.suffix + ".part")
    try:
        with http_client().stream("GET", download["url"]) as response, temporary.open("wb") as out:
            response.raise_for_status()
            for chunk in response.iter_bytes(1024 * 1024):
                out.write(chunk)
        if not zipfile.is_zipfile(temporary):
            raise ValueError(f"Downloaded file is not a valid ZIP: {temporary}")
        temporary.replace(target)
    finally:
        if temporary.exists():
            temporary.unlink()
    print(f"Downloaded national {dataset_type}: {target}")
    return target


def fetch_national_zip(dataset_type: str, year: int, output_dir: Path) -> Path:
    for attempt in range(1, MAX_DOWNLOAD_ATTEMPTS + 1):
        try:
            return fetch_national_zip_once(dataset_type, year, output_dir)
        except DatasetUnavailable:
            raise
        except Exception as exc:
            if not retryable_http_error(exc) or attempt == MAX_DOWNLOAD_ATTEMPTS:
                raise
            delay = min(2 ** (attempt - 1), 16)
            print(
                f"Transient national {dataset_type} download error for year={year}; "
                f"retry {attempt}/{MAX_DOWNLOAD_ATTEMPTS - 1} in {delay}s: {exc}",
                file=sys.stderr,
            )
            time.sleep(delay)
    raise AssertionError("unreachable")


def converted(field: str, value: str):
    value = value.strip()
    if not value:
        return None
    if field in INT_FIELDS:
        return int(value)
    if field in BOOL_FIELDS:
        return value in {"1", "true", "TRUE"}
    if field in DATE_FIELDS:
        if value in NULL_DATE_SENTINELS:
            return None
        return datetime.strptime(value, "%d.%m.%Y").date()
    if field in NUMERIC_FIELDS:
        return value.replace(",", ".")
    return value


def copy_rows(cur, table: str, mapping: dict[str, str], rows, source_file_id: int) -> int:
    preserve_source_rows = table in {"building_parts", "land_parcels", "rental_building_parts"}
    columns = list(mapping.values()) + ["source_file_id"]
    if preserve_source_rows:
        columns.append("source_row_number")
    statement = sql.SQL("COPY {} ({}) FROM STDIN").format(
        sql.Identifier(table), sql.SQL(",").join(map(sql.Identifier, columns)),
    )
    with cur.copy(statement) as copy:
        for row_number, row in enumerate(rows, start=2):
            value = tuple(converted(mapping[source], row.get(source, "")) for source in mapping) + (source_file_id,)
            copy.write_row(value + (row_number,) if preserve_source_rows else value)
    return len(rows)


def import_zip(connection, path: Path) -> ImportStats:
    stats = ImportStats()
    hasher = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            hasher.update(chunk)
    digest = hasher.hexdigest()
    national_match = re.search(r"ETN_SLO_(\d+)_(KPP|NP)", path.name)
    municipality_match = re.search(r"ETN_(\d+)_(\d+)_KPP_", path.name)
    if national_match:
        municipality, year = None, int(national_match.group(1))
        dataset_type, dataset_scope = ("sales" if national_match.group(2) == "KPP" else "rent"), "national"
    elif municipality_match:
        municipality, year = int(municipality_match.group(1)), int(municipality_match.group(2))
        dataset_type, dataset_scope = "sales", "municipality"
    else:
        raise ValueError(f"Cannot determine dataset scope/type/year from archive name: {path.name}")
    selected_tables = RENT_TABLES if dataset_type == "rent" else TABLES
    database_started = time.monotonic()
    with connection.transaction(), connection.cursor() as cur:
        cur.execute("""INSERT INTO source_files(
                           file_name,municipality_code,dataset_year,dataset_type,dataset_scope,sha256,downloaded_at
                       ) VALUES (%s,%s,%s,%s,%s,%s,%s)
                       ON CONFLICT(sha256) DO UPDATE SET file_name=excluded.file_name,
                           dataset_type=excluded.dataset_type,dataset_scope=excluded.dataset_scope
                       RETURNING id""", (
                           path.name, municipality, year, dataset_type, dataset_scope, digest,
                           datetime.fromtimestamp(path.stat().st_mtime, timezone.utc),
                       ))
        source_file_id = cur.fetchone()[0]
        if dataset_type == "rent":
            cur.execute("DELETE FROM rental_transactions WHERE dataset_year=%s", (year,))
        elif dataset_scope == "national":
            cur.execute("DELETE FROM transactions WHERE dataset_year=%s", (year,))
        elif municipality is not None:
            cur.execute("DELETE FROM transactions WHERE municipality_code=%s AND dataset_year=%s", (municipality, year))
        with zipfile.ZipFile(path) as archive:
            members = archive.namelist()
            ordered = sorted(members, key=lambda n: 0 if "_POSLI_" in n else 1)
            for member in ordered:
                parsing_started = time.monotonic()
                payload = archive.read(member)
                rows = list(csv.DictReader(io.StringIO(payload.decode("utf-8-sig"), newline="")))
                stats.parsing_seconds += time.monotonic() - parsing_started
                if "_sifranti_" in member:
                    checksum = hashlib.sha256(payload).hexdigest()
                    cur.execute(
                        """INSERT INTO codebook_imports(checksum,source_file_id) VALUES (%s,%s)
                           ON CONFLICT(checksum) DO NOTHING RETURNING checksum""",
                        (checksum, source_file_id),
                    )
                    if cur.fetchone() is None:
                        stats.codebook_skipped = True
                        print(f"Skipped unchanged codebook ({len(rows)} rows)")
                        continue
                    values = [(int(r["ID"]), r["SIFRANT"], int(r["NUMERICNA_VREDNOST"]), r["OPIS"], source_file_id) for r in rows]
                    cur.executemany("""INSERT INTO codebook_values(codebook_id,codebook_name,numeric_value,description,source_file_id)
                                       VALUES (%s,%s,%s,%s,%s) ON CONFLICT(codebook_id,numeric_value) DO UPDATE SET
                                       codebook_name=excluded.codebook_name,description=excluded.description,source_file_id=excluded.source_file_id""", values)
                    stats.codebook_rows += len(rows)
                    print(f"Imported {len(rows)} rows into codebook_values")
                    continue
                kind = next((key for key in selected_tables if f"_{key}_" in member), None)
                if not kind:
                    raise ValueError(f"Unknown CSV member: {member}")
                table, mapping, _ = selected_tables[kind]
                unknown = set(rows[0] if rows else ()) - set(mapping)
                if unknown:
                    raise ValueError(f"Unmapped columns in {member}: {sorted(unknown)}")
                if "SIFRA_KO" in mapping:
                    historical_cadastral = {
                        (int(row["SIFRA_KO"]), row["IME_KO"].strip())
                        for row in rows if row.get("SIFRA_KO", "").strip()
                    }
                    cur.executemany(
                        """INSERT INTO cadastral_municipalities(code,name) VALUES (%s,%s)
                           ON CONFLICT(code) DO UPDATE SET name=excluded.name""",
                        historical_cadastral,
                    )
                copied = copy_rows(cur, table, mapping, rows, source_file_id)
                stats.rows += copied
                print(f"Copied {copied} rows into {table}")
    stats.database_seconds = max(0.0, time.monotonic() - database_started - stats.parsing_seconds)
    return stats


def prepare_archive(municipality: int, year: int, output_dir: Path, force: bool) -> ArchiveResult:
    started_at = time.monotonic()
    local = sorted(output_dir.glob(f"ETN_{municipality:03d}_{year}_KPP_*.zip"), reverse=True)
    archive = next((path for path in local if zipfile.is_zipfile(path)), None)
    if archive is not None and not force:
        return ArchiveResult(archive, True, time.monotonic() - started_at)
    path = fetch_zip(municipality, year, output_dir)
    return ArchiveResult(path, False, time.monotonic() - started_at)


def prepare_national_archive(dataset_type: str, year: int, output_dir: Path, force: bool) -> ArchiveResult:
    started_at = time.monotonic()
    code = NATIONAL_PRODUCTS[dataset_type]["archive_code"]
    local = sorted(output_dir.glob(f"ETN_SLO_{year}_{code}*.zip"), reverse=True)
    archive = next((path for path in local if zipfile.is_zipfile(path)), None)
    if archive is not None and not force:
        return ArchiveResult(archive, True, time.monotonic() - started_at)
    path = fetch_national_zip(dataset_type, year, output_dir)
    return ArchiveResult(path, False, time.monotonic() - started_at)


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


def refresh_all_sales(
    connection, output_dir: Path, start_year: int, end_year: int, force: bool, workers: int
) -> None:
    started_at = time.monotonic()
    sync_reference_data(connection)
    with connection.cursor() as cur:
        cur.execute("SELECT code FROM municipalities ORDER BY code")
        municipality_codes = [row[0] for row in cur]
        cur.execute("SELECT municipality_code, dataset_year FROM source_files WHERE municipality_code IS NOT NULL")
        already_imported = set(cur.fetchall())
    # End the read transaction so each following archive import is committed independently.
    connection.commit()

    combinations = [
        (code, year)
        for year in range(start_year, end_year + 1)
        for code in municipality_codes
        if force or (code, year) not in already_imported
    ]
    total_possible = len(municipality_codes) * (end_year - start_year + 1)
    skipped = total_possible - len(combinations)
    total = len(combinations)
    imported = unavailable = 0
    download_worker_seconds = parsing_seconds = database_seconds = 0.0
    print(
        f"Queued {total} combinations; skipped {skipped} already imported; "
        f"download workers={workers}; available logical CPU threads={AVAILABLE_THREADS}"
    )
    executor = ThreadPoolExecutor(max_workers=workers, thread_name_prefix="jgp-download")
    combinations_iter = iter(combinations)
    pending: dict[Future, tuple[int, int]] = {}

    def submit_next() -> bool:
        try:
            municipality, year = next(combinations_iter)
        except StopIteration:
            return False
        future = executor.submit(prepare_archive, municipality, year, output_dir, force)
        pending[future] = (municipality, year)
        return True

    for _ in range(min(total, workers * 2)):
        submit_next()

    position = 0
    try:
        while pending:
            completed, _ = wait(pending, return_when=FIRST_COMPLETED)
            for future in completed:
                municipality, year = pending.pop(future)
                position += 1
                label = f"[{position}/{total}] municipality={municipality} year={year}"
                submit_next()
                try:
                    archive = future.result()
                    download_worker_seconds += archive.download_seconds
                    if archive.reused:
                        print(f"{label}: using existing {archive.path}")
                    import_stats = import_zip(connection, archive.path)
                    parsing_seconds += import_stats.parsing_seconds
                    database_seconds += import_stats.database_seconds
                    imported += 1
                    already_imported.add((municipality, year))
                except DatasetUnavailable as exc:
                    unavailable += 1
                    print(f"{label}: unavailable ({exc})", file=sys.stderr)
                except Exception:
                    print(f"{label}: stopped by unexpected error", file=sys.stderr)
                    raise
    finally:
        for future in pending:
            future.cancel()
        executor.shutdown(wait=True, cancel_futures=True)
    elapsed = format_duration(time.monotonic() - started_at)
    print(
        f"Finished: imported={imported}, already_imported={skipped}, "
        f"unavailable={unavailable}, elapsed={elapsed}, "
        f"download_worker_time={format_duration(download_worker_seconds)}, "
        f"csv_parsing={format_duration(parsing_seconds)}, database={format_duration(database_seconds)}"
    )


def refresh_all_national(
    connection, output_dir: Path, end_year: int, force: bool, workers: int
) -> None:
    started_at = time.monotonic()
    sync_reference_data(connection)
    with connection.cursor() as cur:
        cur.execute(
            """SELECT dataset_type,dataset_year FROM source_files
               WHERE dataset_scope='national' AND dataset_type IN ('sales','rent')"""
        )
        already_imported = set(cur.fetchall())
    connection.commit()

    all_combinations = [
        (dataset_type, year)
        for dataset_type, product in NATIONAL_PRODUCTS.items()
        for year in range(product["start_year"], end_year + 1)
    ]
    combinations = [item for item in all_combinations if force or item not in already_imported]
    skipped = len(all_combinations) - len(combinations)
    total = len(combinations)
    imported = unavailable = 0
    download_worker_seconds = parsing_seconds = database_seconds = 0.0
    print(
        f"Queued {total} national yearly archives; skipped {skipped} already imported; "
        f"download workers={workers}; available logical CPU threads={AVAILABLE_THREADS}"
    )

    executor = ThreadPoolExecutor(max_workers=workers, thread_name_prefix="jgp-national-download")
    combinations_iter = iter(combinations)
    pending: dict[Future, tuple[str, int]] = {}

    def submit_next() -> bool:
        try:
            dataset_type, year = next(combinations_iter)
        except StopIteration:
            return False
        future = executor.submit(prepare_national_archive, dataset_type, year, output_dir, force)
        pending[future] = (dataset_type, year)
        return True

    for _ in range(min(total, workers * 2)):
        submit_next()

    position = 0
    try:
        while pending:
            completed, _ = wait(pending, return_when=FIRST_COMPLETED)
            for future in completed:
                dataset_type, year = pending.pop(future)
                position += 1
                label = f"[{position}/{total}] national {dataset_type} year={year}"
                submit_next()
                try:
                    archive = future.result()
                    download_worker_seconds += archive.download_seconds
                    if archive.reused:
                        print(f"{label}: using existing {archive.path}")
                    import_stats = import_zip(connection, archive.path)
                    parsing_seconds += import_stats.parsing_seconds
                    database_seconds += import_stats.database_seconds
                    imported += 1
                    already_imported.add((dataset_type, year))
                except DatasetUnavailable as exc:
                    unavailable += 1
                    print(f"{label}: unavailable ({exc})", file=sys.stderr)
                except Exception:
                    print(f"{label}: stopped by unexpected error", file=sys.stderr)
                    raise
    finally:
        for future in pending:
            future.cancel()
        executor.shutdown(wait=True, cancel_futures=True)

    elapsed = format_duration(time.monotonic() - started_at)
    print(
        f"Finished national import: imported={imported}, already_imported={skipped}, "
        f"unavailable={unavailable}, elapsed={elapsed}, "
        f"download_worker_time={format_duration(download_worker_seconds)}, "
        f"csv_parsing={format_duration(parsing_seconds)}, database={format_duration(database_seconds)}"
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Fetch and import Slovenian JGP real-estate data")
    parser.add_argument("--database-url", default=os.getenv("DATABASE_URL", DEFAULT_DATABASE_URL))
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("sync-reference-data")
    fetch = sub.add_parser("fetch")
    fetch.add_argument("--municipality", type=int, required=True)
    fetch.add_argument("--year", type=int, required=True)
    fetch.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    fetch_national = sub.add_parser("fetch-national", help="Download one national BUY or RENT yearly archive")
    fetch_national.add_argument("--type", choices=tuple(NATIONAL_PRODUCTS), required=True, dest="dataset_type")
    fetch_national.add_argument("--year", type=int, required=True)
    fetch_national.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    load = sub.add_parser("import")
    load.add_argument("zip_file", type=Path)
    refresh = sub.add_parser("refresh")
    refresh.add_argument("--municipality", type=int, required=True)
    refresh.add_argument("--year", type=int, required=True)
    refresh.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    all_sales = sub.add_parser("refresh-all-sales", help="Download and import all municipality/year sales data")
    all_sales.add_argument("--start-year", type=int, default=2007)
    all_sales.add_argument("--end-year", type=int, default=date.today().year)
    all_sales.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    all_sales.add_argument("--force", action="store_true", help="Refresh combinations already present in source_files")
    all_sales.add_argument(
        "--workers", type=int, default=DEFAULT_WORKERS, metavar="N",
        help=f"Parallel download workers (default: {DEFAULT_WORKERS}; machine limit: {AVAILABLE_THREADS})",
    )
    all_national = sub.add_parser(
        "refresh-all-data", help="Download and import all national yearly BUY and RENT archives"
    )
    all_national.add_argument("--end-year", type=int, default=date.today().year)
    all_national.add_argument("--output-dir", type=Path, default=Path("data/raw"))
    all_national.add_argument("--force", action="store_true", help="Refresh national years already imported")
    all_national.add_argument(
        "--workers", type=int, default=DEFAULT_WORKERS, metavar="N",
        help=f"Parallel download workers (default: {DEFAULT_WORKERS}; machine limit: {AVAILABLE_THREADS})",
    )
    args = parser.parse_args()

    if args.command == "fetch":
        fetch_zip(args.municipality, args.year, args.output_dir)
        return
    if args.command == "fetch-national":
        start_year = NATIONAL_PRODUCTS[args.dataset_type]["start_year"]
        if args.year < start_year:
            parser.error(f"{args.dataset_type} national data starts in {start_year}")
        fetch_national_zip(args.dataset_type, args.year, args.output_dir)
        return
    if args.command in {"refresh-all-sales", "refresh-all-data"}:
        if args.command == "refresh-all-sales" and args.start_year > args.end_year:
            parser.error("--start-year must not be later than --end-year")
        if args.command == "refresh-all-data" and args.end_year < NATIONAL_PRODUCTS["sales"]["start_year"]:
            parser.error("--end-year must be 2007 or later")
        try:
            validate_worker_count(args.workers)
        except ValueError as exc:
            parser.error(str(exc))
    with psycopg.connect(args.database_url) as connection:
        if args.command == "sync-reference-data":
            sync_reference_data(connection)
        elif args.command == "import":
            import_zip(connection, args.zip_file)
        elif args.command == "refresh":
            sync_reference_data(connection)
            import_zip(connection, fetch_zip(args.municipality, args.year, args.output_dir))
        elif args.command == "refresh-all-sales":
            refresh_all_sales(connection, args.output_dir, args.start_year, args.end_year, args.force, args.workers)
        elif args.command == "refresh-all-data":
            refresh_all_national(connection, args.output_dir, args.end_year, args.force, args.workers)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)
