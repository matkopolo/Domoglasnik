from real_estate_etl.geography import BoundaryCollection, iter_collection_pages, normalized_feature


def feature(properties=None, geometry_type="Polygon"):
    return {
        "id": "test.1",
        "type": "Feature",
        "properties": properties
        or {"EID_OBCINA": "10", "SIFRA": 70, "NAZIV": "Maribor", "DATUM_SYS": "2026-01-01T00:00:00Z"},
        "geometry": {"type": geometry_type, "coordinates": [[[14.0, 46.0], [15.0, 46.0], [14.0, 46.0]]]},
    }


def test_iter_collection_pages_follows_next_link():
    collection = BoundaryCollection("SI.GURS.RPE:OBCINE", "municipality", "EID_OBCINA")
    responses = [
        {"type": "FeatureCollection", "features": [feature()], "links": [{"rel": "next", "href": "next"}]},
        {"type": "FeatureCollection", "features": [feature()], "links": []},
    ]
    urls = []

    def fetch(url):
        urls.append(url)
        return responses.pop(0)

    pages = list(iter_collection_pages(fetch, collection))
    assert len(pages) == 2
    assert urls[-1] == "next"


def test_iter_collection_pages_rejects_cycle():
    collection = BoundaryCollection("TEST", "municipality", "EID_OBCINA")

    def fetch(url):
        return {"type": "FeatureCollection", "features": [], "links": [{"rel": "next", "href": url}]}

    iterator = iter_collection_pages(fetch, collection)
    next(iterator)
    try:
        next(iterator)
    except ValueError as exc:
        assert "Pagination cycle" in str(exc)
    else:
        raise AssertionError("expected pagination cycle error")


def test_normalized_feature_preserves_parent_and_properties():
    collection = BoundaryCollection(
        "SI.GURS.RPE:CETRTNE_SKUPNOSTI", "city_district", "EID_CETRTNA_SKUPNOST", "EID_OBCINA"
    )
    item = feature(
        {
            "EID_CETRTNA_SKUPNOST": "20",
            "EID_OBCINA": "10",
            "SIFRA": 301,
            "SESTAVLJENA_SIFRA": 70301,
            "NAZIV": "Center",
        },
        geometry_type="MultiPolygon",
    )
    row = normalized_feature(item, collection)
    assert row[0:7] == ("city_district", collection.collection_id, "20", 301, 70301, "Center", "10")


def test_normalized_feature_rejects_non_polygon():
    collection = BoundaryCollection("TEST", "municipality", "EID_OBCINA")
    try:
        normalized_feature(feature(geometry_type="Point"), collection)
    except ValueError as exc:
        assert "Unsupported geometry" in str(exc)
    else:
        raise AssertionError("expected unsupported geometry error")
