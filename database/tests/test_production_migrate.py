import importlib.util
import os
import shutil
import uuid
from pathlib import Path

import psycopg
import pytest
from psycopg import sql

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("production_migrate", ROOT / "production_migrate.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


@pytest.fixture
def database():
    url = os.getenv("MIGRATION_TEST_DATABASE_URL")
    if not url:
        pytest.skip("Requires a dedicated disposable MIGRATION_TEST_DATABASE_URL")
    name = "readiness_test_" + uuid.uuid4().hex
    with psycopg.connect(url, autocommit=True) as admin:
        admin.execute(sql.SQL("CREATE DATABASE {}").format(sql.Identifier(name)))
        try:
            with psycopg.connect(url, dbname=name) as connection:
                yield connection
        finally:
            admin.execute(sql.SQL("DROP DATABASE {} WITH (FORCE)").format(sql.Identifier(name)))


def test_outer_wrappers_preserve_function_blocks():
    assert runner.migration_sql("BEGIN;\nDO $$ BEGIN NULL; END $$;\nCOMMIT;") == (
        "\nDO $$ BEGIN NULL; END $$;\n"
    )


def test_atomic_retry_checksum_and_exclusion(database, tmp_path):
    folder = tmp_path / "migrations"
    folder.mkdir()
    first = folder / "001.sql"
    first.write_text("BEGIN;\nCREATE TABLE marker(id int);\nCOMMIT;")
    second = folder / "002.sql"
    second.write_text("CREATE TABLE rolled_back(id int); SELECT missing_column;")
    with pytest.raises(psycopg.errors.UndefinedColumn):
        runner.migrate(database, tmp_path)
    assert database.execute("SELECT to_regclass('rolled_back')").fetchone()[0] is None
    database.commit()
    second.write_text("CREATE TABLE rolled_back(id int);")
    runner.migrate(database, tmp_path)
    runner.migrate(database, tmp_path)
    assert database.execute("SELECT count(*) FROM production_schema_migrations").fetchone()[0] == 2
    database.commit()
    first.write_text(first.read_text() + "\n-- drift")
    with pytest.raises(RuntimeError, match="modified"):
        runner.migrate(database, tmp_path)
    first.write_text(first.read_text().removesuffix("\n-- drift"))
    (folder / "000.sql").write_text("SELECT 1;")
    with pytest.raises(RuntimeError, match="Out-of-order"):
        runner.migrate(database, tmp_path)
    with psycopg.connect(os.environ["MIGRATION_TEST_DATABASE_URL"], dbname=database.info.dbname) as other:
        other.execute("SELECT pg_advisory_lock(%s)", (runner.LOCK_ID,))
        other.commit()
        with pytest.raises(RuntimeError, match="already running"):
            runner.migrate(database, tmp_path)


def test_full_fresh_schema_and_privileges(database, tmp_path):
    shutil.copytree(ROOT / "migrations", tmp_path / "migrations")
    shutil.copytree(ROOT / "production-migrations", tmp_path / "production-migrations")
    runner.migrate(database, tmp_path)
    runner.migrate(database, tmp_path)
    assert database.execute("SELECT postgis_version()").fetchone()[0]
    assert database.execute(
        "SELECT count(*) FROM production_schema_migrations"
    ).fetchone()[0] == len(list(tmp_path.rglob("*.sql")))
    for role in ("real_estate_api", "real_estate_pipeline"):
        assert not database.execute(
            "SELECT has_schema_privilege(%s,'analytics','CREATE')", (role,)
        ).fetchone()[0]
    assert not database.execute(
        "SELECT has_table_privilege('real_estate_api','transactions','INSERT')"
    ).fetchone()[0]
    assert not database.execute(
        "SELECT has_function_privilege('real_estate_api','analytics.refresh_sale_facts()','EXECUTE')"
    ).fetchone()[0]
    assert database.execute(
        "SELECT has_table_privilege('real_estate_pipeline','transactions','INSERT')"
    ).fetchone()[0]
    database.execute("SET ROLE real_estate_pipeline")
    # Empty but real functions exercise write/sequence/temporary-table permissions.
    database.execute("SELECT analytics.refresh_sale_facts()")
    database.execute("SELECT analytics.refresh_rental_facts()")
    database.execute("RESET ROLE")
    assert database.execute(
        "SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace "
        "WHERE n.nspname='analytics' AND p.proname LIKE 'refresh_%' AND NOT p.prosecdef"
    ).fetchone()[0] == 0
