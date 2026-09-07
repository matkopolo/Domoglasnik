"""Fresh-production migrations: session lock, atomic files and SHA-256 history.

The Compose bootstrap remains unchanged. Never adopt an existing untracked
database automatically; restore/export through the documented migration path.
"""

import hashlib
import logging
import os
import re
from pathlib import Path

import psycopg

LOCK_ID = 724_202_607  # Same exclusion boundary as scheduled/manual pipeline work.
LOGGER = logging.getLogger("migrations")


def migration_sql(source):
    # Only the three historical outer transaction wrappers are removed.
    # PL/pgSQL BEGIN blocks are untouched. The driver owns the atomic transaction.
    return re.sub(r"^(?:BEGIN|COMMIT);\s*$", "", source, flags=re.MULTILINE)


def migrate(connection, root):
    files = sorted((root / "migrations").glob("*.sql"))
    files += sorted((root / "production-migrations").glob("*.sql"))
    if not files:
        raise RuntimeError("No migrations found")
    names = [str(path.relative_to(root)) for path in files]
    if not connection.execute("SELECT pg_try_advisory_lock(%s)", (LOCK_ID,)).fetchone()[0]:
        raise RuntimeError("Migration or pipeline maintenance already running")
    connection.commit()
    try:
        exists = connection.execute(
            "SELECT to_regclass('public.production_schema_migrations')"
        ).fetchone()[0]
        if not exists and connection.execute(
            "SELECT to_regclass('public.transactions')"
        ).fetchone()[0]:
            raise RuntimeError("Untracked existing database: explicit adoption is required")
        connection.execute("""
            CREATE TABLE IF NOT EXISTS public.production_schema_migrations(
                filename text PRIMARY KEY, sha256 text NOT NULL,
                applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
            )
        """)
        applied = dict(connection.execute(
            "SELECT filename,sha256 FROM public.production_schema_migrations"
        ).fetchall())
        for name, checksum in applied.items():
            if name not in names or hashlib.sha256((root / name).read_bytes()).hexdigest() != checksum:
                raise RuntimeError(f"Applied migration removed or modified: {name}")
        for directory in ("migrations/", "production-migrations/"):
            latest = max((name for name in applied if name.startswith(directory)), default="")
            if any(name.startswith(directory) and name not in applied and name < latest for name in names):
                raise RuntimeError(f"Out-of-order migration in {directory}")
        # CNPG normally manages these roles before bootstrap. Missing roles are
        # NOLOGIN, preventing migration 013's development-password fallback.
        connection.execute("""
            DO $$ BEGIN
              IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='real_estate') THEN
                CREATE ROLE real_estate NOLOGIN;
              END IF;
              IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='real_estate_api') THEN
                CREATE ROLE real_estate_api NOLOGIN;
              END IF;
              IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='real_estate_pipeline') THEN
                CREATE ROLE real_estate_pipeline NOLOGIN;
              END IF;
            END $$;
        """)
        connection.commit()
        for path, name in zip(files, names):
            if name in applied:
                LOGGER.info("Already applied: %s", name)
                continue
            with connection.transaction():
                connection.execute(migration_sql(path.read_text()), prepare=False)
                connection.execute(
                    "INSERT INTO public.production_schema_migrations(filename,sha256) VALUES(%s,%s)",
                    (name, hashlib.sha256(path.read_bytes()).hexdigest()),
                )
            LOGGER.info("Applied: %s", name)
    finally:
        connection.rollback()
        connection.execute("SELECT pg_advisory_unlock(%s)", (LOCK_ID,))
        connection.commit()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    with psycopg.connect(os.environ["DATABASE_URL"]) as database:
        migrate(database, Path(__file__).parent)
