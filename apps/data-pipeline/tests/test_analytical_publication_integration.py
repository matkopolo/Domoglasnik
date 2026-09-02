import os
import uuid

import psycopg
import pytest


def test_active_release_switch_is_invisible_until_commit():
    database_url = os.getenv("TEST_DATABASE_URL")
    if not database_url:
        pytest.skip("TEST_DATABASE_URL is not set")

    pending_id = uuid.uuid4()
    with psycopg.connect(database_url) as setup:
        previous = setup.execute(
            "SELECT active_release_id FROM analytics.analytical_publication WHERE singleton"
        ).fetchone()[0]
        setup.execute(
            """INSERT INTO analytics.analytical_releases(
                   id,status,stage,source_sale_through,source_rent_through,published_at
               ) VALUES(%s,'published','published',DATE '2026-06-30',DATE '2026-06-30',
                        clock_timestamp())""",
            (pending_id,),
        )
        setup.commit()

    try:
        with psycopg.connect(database_url) as writer, psycopg.connect(database_url) as reader:
            writer.execute(
                """UPDATE analytics.analytical_publication
                   SET active_release_id=%s WHERE singleton""",
                (pending_id,),
            )
            assert reader.execute(
                "SELECT active_release_id FROM analytics.analytical_publication WHERE singleton"
            ).fetchone()[0] == previous
            reader.commit()
            writer.commit()
            assert reader.execute(
                "SELECT active_release_id FROM analytics.analytical_publication WHERE singleton"
            ).fetchone()[0] == pending_id
    finally:
        with psycopg.connect(database_url) as cleanup:
            cleanup.execute(
                "UPDATE analytics.analytical_publication SET active_release_id=%s WHERE singleton",
                (previous,),
            )
            cleanup.execute(
                "DELETE FROM analytics.analytical_releases WHERE id=%s", (pending_id,)
            )
