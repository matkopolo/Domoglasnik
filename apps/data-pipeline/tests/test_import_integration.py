import csv
import io
import os
import uuid
import zipfile
from pathlib import Path

import psycopg
import pytest
from psycopg import sql

from real_estate_etl.importer import IGNORED_SOURCE_COLUMNS, RENT_TABLES, TABLES, import_zip

DATABASE_URL = os.getenv("TEST_DATABASE_URL")
pytestmark = pytest.mark.integration


def test_public_source_free_text_is_not_mapped_for_persistence():
    source_columns = {
        *TABLES["POSLI"][1],
        *TABLES["DELISTAVB"][1],
        *TABLES["ZEMLJISCA"][1],
        *RENT_TABLES["POSLI"][1],
        *RENT_TABLES["DELISTAVB"][1],
    }
    assert source_columns.isdisjoint({
        "OPOMBE_O_PRAVNEM_POSLU",
        "OPOMBE_O_NEPREMICNINI",
        "OPOMBE_O_ODDANIH_PROSTORIH",
    })
    assert {
        "OPOMBE_O_PRAVNEM_POSLU",
        "OPOMBE_O_NEPREMICNINI",
        "OPOMBE_O_ODDANIH_PROSTORIH",
    } == IGNORED_SOURCE_COLUMNS


def csv_bytes(fieldnames, rows):
    output = io.StringIO(newline="")
    writer = csv.DictWriter(output, fieldnames=fieldnames)
    writer.writeheader()
    writer.writerows(rows)
    return output.getvalue().encode("utf-8")


def complete_row(kind, values):
    mapping = TABLES[kind][1]
    return {source: values.get(source, "") for source in mapping}


@pytest.fixture
def database_connection():
    if not DATABASE_URL:
        pytest.skip("Set TEST_DATABASE_URL to run PostgreSQL integration tests")
    schema = f"test_etl_{uuid.uuid4().hex}"
    admin = psycopg.connect(DATABASE_URL, autocommit=True)
    admin.execute(sql.SQL("CREATE SCHEMA {}").format(sql.Identifier(schema)))
    connection = psycopg.connect(DATABASE_URL, options=f"-c search_path={schema},public")
    repository_root = Path(__file__).resolve().parents[3]
    connection.execute((repository_root / "database/migrations/001_schema.sql").read_text())
    connection.execute((repository_root / "database/migrations/007_postgis.sql").read_text())
    connection.commit()
    try:
        yield connection
    finally:
        connection.close()
        admin.execute(sql.SQL("DROP SCHEMA {} CASCADE").format(sql.Identifier(schema)))
        admin.close()


def create_historical_archive(path: Path):
    transaction = complete_row("POSLI", {
        "ID_POSLA": "-1", "LETO": "2009", "RPE_OBCINE_SIFRA": "999",
        "DATUM_SKLENITVE_POGODBE": "09.01.2009", "POGODBENA_CENA_ODSKODNINA": "100000",
    })
    building_base = {
        "ID_POSLA": "-1", "SIFRA_KO": "3986", "IME_KO": "HISTORICAL",
        "LETO": "2009", "RPE_OBCINE_SIFRA": "999", "STEVILKA_STAVBE": "1",
        "STEVILKA_DELA_STAVBE": "1",
    }
    buildings = [
        complete_row("DELISTAVB", {**building_base, "PRODANA_POVRSINA": "40"}),
        complete_row("DELISTAVB", {**building_base, "PRODANA_POVRSINA": "50"}),
        complete_row("DELISTAVB", {
            "ID_POSLA": "-1", "LETO": "2009", "RPE_OBCINE_SIFRA": "999",
            "EVIDENTIRANOST_DELA_STAVBE": "0",
        }),
    ]
    lands = [
        complete_row("ZEMLJISCA", {
            "ID_POSLA": "-1", "SIFRA_KO": "3986", "IME_KO": "HISTORICAL",
            "PARCELNA_STEVILKA": "10/2", "LETO": "2009", "RPE_OBCINE_SIFRA": "999",
        }),
        complete_row("ZEMLJISCA", {
            "ID_POSLA": "-1", "VRSTA_ZEMLJISCA": "3", "PRODANI_DELEZ_PARCELE": "NP",
            "LETO": "2009", "RPE_OBCINE_SIFRA": "999",
        }),
    ]
    codebook = [{"ID": "3", "SIFRANT": "Vrsta zemljišča", "NUMERICNA_VREDNOST": "1", "OPIS": "Test"}]
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("ETN_999_2009_KPP_2009_POSLI_test.csv", csv_bytes(TABLES["POSLI"][1], [transaction]))
        archive.writestr("ETN_999_2009_KPP_sifranti_test.csv", csv_bytes(codebook[0], codebook))
        archive.writestr("ETN_999_2009_KPP_2009_DELISTAVB_test.csv", csv_bytes(TABLES["DELISTAVB"][1], buildings))
        archive.writestr("ETN_999_2009_KPP_2009_ZEMLJISCA_test.csv", csv_bytes(TABLES["ZEMLJISCA"][1], lands))


def create_national_sales_archive(path: Path):
    transaction = complete_row("POSLI", {"ID_POSLA": "700", "LETO": "2010"})
    building = complete_row("DELISTAVB", {
        "ID_POSLA": "700", "SIFRA_KO": "3986", "IME_KO": "HISTORICAL", "OBCINA": "TEST TOWN",
        "STEVILKA_STAVBE": "8", "E_CENTROID": "461000", "N_CENTROID": "102000", "LETO": "2010",
    })
    land = complete_row("ZEMLJISCA", {
        "ID_POSLA": "700", "OBCINA": "TEST TOWN", "PARCELNA_STEVILKA": "1/1", "LETO": "2010",
    })
    transaction_fields = [x for x in TABLES["POSLI"][1] if x != "RPE_OBCINE_SIFRA"]
    building_fields = [x for x in TABLES["DELISTAVB"][1] if x != "RPE_OBCINE_SIFRA"]
    land_fields = [x for x in TABLES["ZEMLJISCA"][1] if x != "RPE_OBCINE_SIFRA"]
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("ETN_SLO_2010_KPP_KPP_POSLI_test.csv", csv_bytes(
            transaction_fields, [{key: transaction[key] for key in transaction_fields}]
        ))
        archive.writestr("ETN_SLO_2010_KPP_KPP_DELISTAVB_test.csv", csv_bytes(
            building_fields, [{key: building[key] for key in building_fields}]
        ))
        archive.writestr("ETN_SLO_2010_KPP_KPP_ZEMLJISCA_test.csv", csv_bytes(
            land_fields, [{key: land[key] for key in land_fields}]
        ))


def create_national_rent_archive(path: Path):
    transaction_mapping = RENT_TABLES["POSLI"][1]
    building_mapping = RENT_TABLES["DELISTAVB"][1]
    transaction = {source: "" for source in transaction_mapping}
    transaction.update({"ID_POSLA": "800", "LETO": "2013", "POGODBENA_NAJEMNINA": "500"})
    building = {source: "" for source in building_mapping}
    building.update({
        "ID_POSLA": "800", "OBCINA": "TEST TOWN", "LETO": "2013",
        "POVRSINA_ODDANIH_PROSTOROV": "44.5", "GOSTINSKI_VRT": "0",
    })
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("ETN_SLO_2013_NP_NP_POSLI_test.csv", csv_bytes(transaction_mapping, [transaction]))
        archive.writestr("ETN_SLO_2013_NP_NP_DELISTAVB_test.csv", csv_bytes(building_mapping, [building]))


def test_import_preserves_historical_incomplete_and_duplicate_rows(database_connection, tmp_path):
    connection = database_connection
    connection.execute("INSERT INTO municipalities(code,name) VALUES (999,'TEST')")
    connection.commit()
    archive = tmp_path / "ETN_999_2009_KPP_test.zip"
    create_historical_archive(archive)

    first = import_zip(connection, archive)
    second = import_zip(connection, archive)

    assert first.rows == 6
    assert first.codebook_rows == 1
    assert second.unchanged is True
    assert connection.execute("SELECT count(*) FROM transactions").fetchone()[0] == 1
    assert connection.execute("SELECT count(*) FROM building_parts").fetchone()[0] == 3
    assert connection.execute("SELECT count(*) FROM land_parcels").fetchone()[0] == 2
    assert connection.execute("SELECT count(*) FROM codebook_imports").fetchone()[0] == 1
    assert connection.execute(
        "SELECT count(*) FROM land_parcels WHERE cadastral_municipality_code IS NULL"
    ).fetchone()[0] == 1
    assert connection.execute(
        "SELECT count(*) FROM building_parts WHERE building_part_number IS NULL"
    ).fetchone()[0] == 1
    assert connection.execute(
        "SELECT name FROM cadastral_municipalities WHERE code=3986"
    ).fetchone()[0] == "HISTORICAL"


def test_national_sales_and_rent_snapshots(database_connection, tmp_path):
    connection = database_connection
    sales = tmp_path / "ETN_SLO_2010_KPP_test.zip"
    rent = tmp_path / "ETN_SLO_2013_NP_test.zip"
    create_national_sales_archive(sales)
    create_national_rent_archive(rent)

    sales_stats = import_zip(connection, sales)
    rent_stats = import_zip(connection, rent)

    assert sales_stats.rows == 3
    assert rent_stats.rows == 2
    assert connection.execute(
        "SELECT municipality_code FROM transactions WHERE transaction_id=700"
    ).fetchone()[0] is None
    assert connection.execute(
        "SELECT dataset_type,dataset_scope FROM source_files WHERE dataset_year=2010"
    ).fetchone() == ("sales", "national")
    assert connection.execute(
        "SELECT ST_SRID(location) FROM building_parts WHERE transaction_id=700"
    ).fetchone()[0] == 3794
    longitude, latitude = connection.execute(
        "SELECT ST_X(ST_Transform(location,4326)), ST_Y(ST_Transform(location,4326)) "
        "FROM building_parts WHERE transaction_id=700"
    ).fetchone()
    assert 13.0 < longitude < 17.0
    assert 45.0 < latitude < 47.0
    assert connection.execute("SELECT contract_rent FROM rental_transactions WHERE transaction_id=800").fetchone()[0] == 500
    assert connection.execute("SELECT municipality_name FROM rental_building_parts").fetchone()[0] == "TEST TOWN"


def test_import_streams_zip_members_without_zipfile_read(
    database_connection, tmp_path, monkeypatch
):
    archive = tmp_path / "ETN_SLO_2010_KPP_test.zip"
    create_national_sales_archive(archive)

    def forbid_materializing_member(*_args, **_kwargs):
        raise AssertionError("ZipFile.read materializes the complete member")

    monkeypatch.setattr(zipfile.ZipFile, "read", forbid_materializing_member)
    stats = import_zip(database_connection, archive)
    assert stats.rows == 3
