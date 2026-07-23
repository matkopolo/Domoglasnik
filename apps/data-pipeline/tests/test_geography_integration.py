import os
import uuid
from pathlib import Path

import psycopg
import pytest
from psycopg import sql

from real_estate_etl.geography import COLLECTIONS, sync_geographies

DATABASE_URL = os.getenv("TEST_DATABASE_URL")
pytestmark = pytest.mark.integration


def polygon(west, south, east, north):
    return {
        "type": "Polygon",
        "coordinates": [[[west, south], [east, south], [east, north], [west, north], [west, south]]],
    }


def boundary_feature(collection, eid, name, geometry, parent_eid=None):
    properties = {collection.eid_field: eid, "SIFRA": 1, "NAZIV": name, "DATUM_SYS": "2026-01-01T00:00:00Z"}
    if collection.parent_eid_field:
        properties[collection.parent_eid_field] = parent_eid
        properties["SESTAVLJENA_SIFRA"] = 1001
    return {"type": "Feature", "id": eid, "properties": properties, "geometry": geometry}


@pytest.fixture
def analytics_connection():
    if not DATABASE_URL:
        pytest.skip("Set TEST_DATABASE_URL to run PostgreSQL integration tests")
    schema = f"test_analytics_{uuid.uuid4().hex}"
    connection = psycopg.connect(DATABASE_URL)
    repository_root = Path(__file__).resolve().parents[3]
    migration = (repository_root / "database/migrations/008_api_foundation.sql").read_text().replace(
        "analytics", schema
    )
    connection.execute(migration)
    connection.commit()
    try:
        yield connection, schema
    finally:
        connection.close()
        admin = psycopg.connect(DATABASE_URL, autocommit=True)
        admin.execute(sql.SQL("DROP SCHEMA {} CASCADE").format(sql.Identifier(schema)))
        admin.close()


def test_sync_geographies_populates_hierarchy_and_audit(analytics_connection):
    connection, schema = analytics_connection
    region, municipality, district, community, settlement = COLLECTIONS
    features = {
        region.collection_id: [boundary_feature(region, "region-1", "Test region", polygon(13, 45, 17, 47))],
        municipality.collection_id: [
            boundary_feature(municipality, "municipality-1", "Test municipality", polygon(14, 46, 15, 46.5))
        ],
        district.collection_id: [
            boundary_feature(district, "district-1", "Center", polygon(14.1, 46.1, 14.4, 46.3), "municipality-1")
        ],
        community.collection_id: [
            boundary_feature(
                community, "community-1", "Community", polygon(14.4, 46.1, 14.7, 46.3), "municipality-1"
            )
        ],
        settlement.collection_id: [
            boundary_feature(
                settlement, "settlement-1", "Settlement", polygon(14.2, 46.2, 14.3, 46.25), "municipality-1"
            )
        ],
    }

    def fetch(url):
        collection_id = next(item.collection_id for item in COLLECTIONS if item.collection_id.replace(":", "%3A") in url)
        return {"type": "FeatureCollection", "features": features[collection_id], "links": []}

    counts = sync_geographies(connection, fetch_json=fetch, schema=schema)
    assert counts == {
        "statistical_region": 1,
        "municipality": 1,
        "city_district": 1,
        "local_community": 1,
        "settlement": 1,
        "country": 1,
    }
    assert connection.execute(sql.SQL("SELECT count(*) FROM {}.geographic_units").format(sql.Identifier(schema))).fetchone()[0] == 6
    hierarchy = connection.execute(
        sql.SQL(
            """SELECT child.level,parent.level
                 FROM {}.geographic_units child
                 LEFT JOIN {}.geographic_units parent ON parent.id=child.parent_id
                ORDER BY child.level"""
        ).format(sql.Identifier(schema), sql.Identifier(schema))
    ).fetchall()
    assert ("municipality", "statistical_region") in hierarchy
    assert ("city_district", "municipality") in hierarchy
    assert ("statistical_region", "country") in hierarchy
    assert connection.execute(
        sql.SQL("SELECT status,details->'counts'->>'country' FROM {}.refresh_runs").format(sql.Identifier(schema))
    ).fetchone() == ("succeeded", "1")
