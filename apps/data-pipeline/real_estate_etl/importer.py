"""CSV/ZIP conversion and PostgreSQL import for GURS transaction archives."""

from __future__ import annotations

import csv
import hashlib
import io
import re
import time
import zipfile
from datetime import datetime, timezone
from pathlib import Path

from psycopg import sql

from .conversion import converted
from .models import ImportStats

IGNORED_SOURCE_COLUMNS = {
    "OPOMBE_O_PRAVNEM_POSLU",
    "OPOMBE_O_NEPREMICNINI",
    "OPOMBE_O_ODDANIH_PROSTORIH",
}

TABLES = {
    "POSLI": ("transactions", {
        "ID_POSLA": "transaction_id", "VRSTA_KUPOPRODAJNEGA_POSLA": "transaction_type",
        "DATUM_UVELJAVITVE": "effective_date", "DATUM_SKLENITVE_POGODBE": "contract_date",
        "POGODBENA_CENA_ODSKODNINA": "contract_price", "VKLJUCENOST_DDV": "vat_included",
        "STOPNJA_DDV": "vat_rate", "DATUM_IZTEKA_LIZINGA": "lease_expiry_date",
        "DATUM_PRENEHANJA_LIZINGA": "lease_termination_date",
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
        "POVRSINA_ATRIJA": "atrium_area",
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
        "PRODANI_DELEZ_PARCELE": "sold_share",
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

def copy_rows(
    cur, table: str, mapping: dict[str, str], rows, source_file_id: int,
    *, archive_name: str = "<unknown archive>", member: str = "<unknown member>",
) -> int:
    preserve_source_rows = table in {"building_parts", "land_parcels", "rental_building_parts"}
    columns = list(mapping.values()) + ["source_file_id"]
    if preserve_source_rows:
        columns.append("source_row_number")
    statement = sql.SQL("COPY {} ({}) FROM STDIN").format(
        sql.Identifier(table), sql.SQL(",").join(map(sql.Identifier, columns)),
    )
    count = 0
    with cur.copy(statement) as copy:
        for row_number, row in enumerate(rows, start=2):
            value = tuple(
                converted(
                    mapping[source], row.get(source, ""),
                    archive=archive_name, member=member, row_number=row_number,
                    source_field=source,
                )
                for source in mapping
            ) + (source_file_id,)
            copy.write_row(value + (row_number,) if preserve_source_rows else value)
            count += 1
    return count


def _reader(archive: zipfile.ZipFile, member: str):
    binary = archive.open(member)
    text = io.TextIOWrapper(binary, encoding="utf-8-sig", newline="")
    return text, csv.DictReader(text)


def _member_checksum(archive: zipfile.ZipFile, member: str) -> str:
    hasher = hashlib.sha256()
    with archive.open(member) as source:
        while chunk := source.read(1024 * 1024):
            hasher.update(chunk)
    return hasher.hexdigest()


def _historical_cadastral_units(
    archive: zipfile.ZipFile, member: str,
) -> set[tuple[int, str]]:
    text, rows = _reader(archive, member)
    try:
        return {
            (int(row["SIFRA_KO"]), row.get("IME_KO", "").strip())
            for row in rows
            if row.get("SIFRA_KO", "").strip()
        }
    finally:
        text.close()


def _import_codebook(
    cur, archive: zipfile.ZipFile, member: str, source_file_id: int,
) -> int:
    count = 0
    batch = []
    text, rows = _reader(archive, member)
    try:
        for row in rows:
            batch.append((
                int(row["ID"]), row["SIFRANT"], int(row["NUMERICNA_VREDNOST"]),
                row["OPIS"], source_file_id,
            ))
            if len(batch) == 500:
                _write_codebook_batch(cur, batch)
                count += len(batch)
                batch.clear()
        if batch:
            _write_codebook_batch(cur, batch)
            count += len(batch)
    finally:
        text.close()
    return count


def _write_codebook_batch(cur, values) -> None:
    cur.executemany(
        """INSERT INTO codebook_values(
               codebook_id,codebook_name,numeric_value,description,source_file_id
           ) VALUES (%s,%s,%s,%s,%s)
           ON CONFLICT(codebook_id,numeric_value) DO UPDATE SET
             codebook_name=excluded.codebook_name,
             description=excluded.description,
             source_file_id=excluded.source_file_id""",
        values,
    )


def import_zip(connection, path: Path, verbose: bool = False) -> ImportStats:
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
                if "_sifranti_" in member:
                    checksum = _member_checksum(archive, member)
                    cur.execute(
                        """INSERT INTO codebook_imports(checksum,source_file_id) VALUES (%s,%s)
                           ON CONFLICT(checksum) DO NOTHING RETURNING checksum""",
                        (checksum, source_file_id),
                    )
                    if cur.fetchone() is None:
                        stats.codebook_skipped = True
                        if verbose:
                            print("Skipped unchanged codebook")
                        continue
                    imported = _import_codebook(cur, archive, member, source_file_id)
                    stats.codebook_rows += imported
                    stats.parsing_seconds += time.monotonic() - parsing_started
                    if verbose:
                        print(f"Imported {imported} rows into codebook_values")
                    continue
                kind = next((key for key in selected_tables if f"_{key}_" in member), None)
                if not kind:
                    raise ValueError(f"Unknown CSV member: {member}")
                table, mapping, _ = selected_tables[kind]
                if "SIFRA_KO" in mapping:
                    historical_cadastral = _historical_cadastral_units(archive, member)
                    cur.executemany(
                        """INSERT INTO cadastral_municipalities(code,name) VALUES (%s,%s)
                           ON CONFLICT(code) DO UPDATE SET name=excluded.name""",
                        historical_cadastral,
                    )
                text, rows = _reader(archive, member)
                try:
                    unknown = set(rows.fieldnames or ()) - set(mapping) - IGNORED_SOURCE_COLUMNS
                    if unknown:
                        raise ValueError(
                            f"Unmapped columns in {member}: {sorted(unknown)}"
                        )
                    copied = copy_rows(
                        cur, table, mapping, rows, source_file_id,
                        archive_name=path.name, member=member,
                    )
                finally:
                    text.close()
                stats.parsing_seconds += time.monotonic() - parsing_started
                stats.rows += copied
                if verbose:
                    print(f"Copied {copied} rows into {table}")
    stats.database_seconds = max(0.0, time.monotonic() - database_started - stats.parsing_seconds)
    return stats
