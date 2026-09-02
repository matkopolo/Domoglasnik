import json
import os
import uuid

import psycopg
import pytest


def _model_run(connection, run_id, version):
    connection.execute(
        """INSERT INTO analytics.model_runs(
               id,transaction_kind,model_version,model_type,feature_schema,
               training_rows,metrics,random_seed
           ) VALUES(%s,'sale',%s,'test','{}'::jsonb,1,%s::jsonb,42)""",
        (run_id, version, json.dumps({"test": True})),
    )


def test_pending_model_generation_is_invisible_until_activation_commits():
    database_url = os.getenv("TEST_DATABASE_URL")
    if not database_url:
        pytest.skip("TEST_DATABASE_URL is not set")
    old_run, new_run = uuid.uuid4(), uuid.uuid4()
    transaction_id = -9_000_000_036
    with psycopg.connect(database_url) as setup:
        previous = setup.execute(
            "SELECT model_run_id FROM analytics.model_publications WHERE transaction_kind='sale'"
        ).fetchone()
        _model_run(setup, old_run, f"test-old-{old_run}")
        _model_run(setup, new_run, f"test-new-{new_run}")
        for run_id, value in ((old_run, 1000), (new_run, 2000)):
            setup.execute(
                """INSERT INTO analytics.valuation_predictions(
                       transaction_kind,transaction_id,model_run_id,
                       predicted_amount_per_sqm,lower_amount_per_sqm,
                       upper_amount_per_sqm,actual_amount_per_sqm,log_residual
                   ) VALUES('sale',%s,%s,%s,%s,%s,%s,0)""",
                (transaction_id, run_id, value, value - 100, value + 100, value),
            )
        setup.execute(
            """INSERT INTO analytics.model_publications(transaction_kind,model_run_id)
               VALUES('sale',%s) ON CONFLICT(transaction_kind) DO UPDATE
               SET model_run_id=excluded.model_run_id""",
            (old_run,),
        )
        setup.commit()
    try:
        with psycopg.connect(database_url) as writer, psycopg.connect(database_url) as reader:
            writer.execute(
                "UPDATE analytics.model_publications SET model_run_id=%s WHERE transaction_kind='sale'",
                (new_run,),
            )
            assert reader.execute(
                """SELECT predicted_amount_per_sqm FROM analytics.current_valuation_predictions
                   WHERE transaction_kind='sale' AND transaction_id=%s""",
                (transaction_id,),
            ).fetchone()[0] == 1000
            reader.commit()
            writer.commit()
            assert reader.execute(
                """SELECT predicted_amount_per_sqm FROM analytics.current_valuation_predictions
                   WHERE transaction_kind='sale' AND transaction_id=%s""",
                (transaction_id,),
            ).fetchone()[0] == 2000
    finally:
        with psycopg.connect(database_url) as cleanup:
            if previous:
                cleanup.execute(
                    "UPDATE analytics.model_publications SET model_run_id=%s WHERE transaction_kind='sale'",
                    (previous[0],),
                )
            else:
                cleanup.execute("DELETE FROM analytics.model_publications WHERE transaction_kind='sale'")
            cleanup.execute(
                "DELETE FROM analytics.valuation_predictions WHERE model_run_id IN (%s,%s)",
                (old_run, new_run),
            )
            cleanup.execute("DELETE FROM analytics.model_runs WHERE id IN (%s,%s)", (old_run, new_run))
