"""Strict source-value conversion with row-level diagnostics."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal, InvalidOperation

INT_FIELDS = {
    "transaction_id", "transaction_type", "deed_type", "marketability",
    "dataset_year", "municipality_code", "cadastral_municipality_code",
    "recorded_status", "building_part_type", "construction_year",
    "construction_phase", "outdoor_parking_spaces", "land_type",
    "permanent_crop_type", "permanent_crop_age", "rental_transaction_type",
    "rented_spaces_type", "furnishing", "microlocation",
}
BOOL_FIELDS = {
    "vat_included", "agency_brokered", "building_completed", "new_construction",
    "atrium", "operating_costs_included", "storefront", "shopping_center",
    "rented_area_equals_building_part", "hospitality_garden",
    "garden_included_in_rent",
}
DATE_FIELDS = {
    "effective_date", "contract_date", "lease_expiry_date",
    "lease_termination_date", "last_changed_date", "last_effective_date",
    "rental_start_date", "rental_end_date", "closure_or_early_termination_date",
}
NULL_DATE_SENTINELS = {"00.00.0000", "0.0.0000", "00.00.00"}
NUMERIC_FIELDS = {
    "contract_price", "vat_rate", "sold_area", "sold_building_part_area",
    "sold_usable_area", "atrium_area", "room_count", "building_part_area",
    "usable_area", "contractual_price", "centroid_e", "centroid_n",
    "parcel_area", "contract_rent", "rented_spaces_area",
    "usable_building_part_area", "rented_usable_area", "hospitality_garden_area",
}
TRUE_TOKENS = {"1", "true"}
FALSE_TOKENS = {"0", "false"}


@dataclass(frozen=True)
class SourceConversionError(ValueError):
    archive: str
    member: str
    row_number: int
    source_field: str
    destination_field: str
    raw_value: str
    reason: str

    def __str__(self) -> str:
        return (
            f"{self.archive}:{self.member}: row {self.row_number}, "
            f"{self.source_field}->{self.destination_field}, value "
            f"{self.raw_value!r}: {self.reason}"
        )


def converted(
    field: str,
    value: str,
    *,
    archive: str = "<unknown archive>",
    member: str = "<unknown member>",
    row_number: int = 0,
    source_field: str = "<unknown field>",
):
    stripped = value.strip()
    if not stripped:
        return None
    try:
        if field in INT_FIELDS:
            return int(stripped)
        if field in BOOL_FIELDS:
            token = stripped.lower()
            if token in TRUE_TOKENS:
                return True
            if token in FALSE_TOKENS:
                return False
            raise ValueError("unsupported boolean token")
        if field in DATE_FIELDS:
            if stripped in NULL_DATE_SENTINELS:
                return None
            return datetime.strptime(stripped, "%d.%m.%Y").date()
        if field in NUMERIC_FIELDS:
            return Decimal(stripped.replace(",", "."))
        return stripped
    except (ValueError, InvalidOperation) as error:
        raise SourceConversionError(
            archive=archive,
            member=member,
            row_number=row_number,
            source_field=source_field,
            destination_field=field,
            raw_value=value,
            reason=str(error),
        ) from error
