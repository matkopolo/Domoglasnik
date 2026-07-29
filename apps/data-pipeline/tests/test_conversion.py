from datetime import date
from decimal import Decimal

import pytest

from real_estate_etl.conversion import SourceConversionError, converted


@pytest.mark.parametrize(
    ("field", "raw", "expected"),
    [
        ("vat_included", "1", True),
        ("vat_included", "TRUE", True),
        ("vat_included", "0", False),
        ("vat_included", "false", False),
        ("contract_date", "09.01.2026", date(2026, 1, 9)),
        ("contract_date", "00.00.0000", None),
        ("contract_price", "12,50", Decimal("12.50")),
        ("transaction_id", "-42", -42),
        ("parcel_number", " 10/2 ", "10/2"),
    ],
)
def test_conversion_contract(field, raw, expected):
    assert converted(field, raw) == expected


@pytest.mark.parametrize(
    ("field", "raw"),
    [
        ("vat_included", "yes"),
        ("transaction_id", "12.5"),
        ("contract_price", "twelve"),
        ("contract_date", "2026-01-09"),
    ],
)
def test_conversion_error_contains_source_context(field, raw):
    with pytest.raises(SourceConversionError) as captured:
        converted(
            field,
            raw,
            archive="ETN_SLO_2026_KPP.zip",
            member="ETN_POSLI.csv",
            row_number=17,
            source_field="SOURCE_FIELD",
        )
    error = captured.value
    assert error.archive == "ETN_SLO_2026_KPP.zip"
    assert error.member == "ETN_POSLI.csv"
    assert error.row_number == 17
    assert error.source_field == "SOURCE_FIELD"
    assert error.destination_field == field
    assert error.raw_value == raw
    assert "row 17" in str(error)
