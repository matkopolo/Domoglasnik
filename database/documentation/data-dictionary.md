# Real-estate database data dictionary

This document describes the PostgreSQL schema populated from the Slovenian Surveying and Mapping Authority (GURS) JGP national BUY (`KPP`) and RENT (`NP`) archives.

## Conventions

- Source field names are the original Slovenian CSV headers.
- Monetary values are stored as `numeric` and are normally euros, as supplied by GURS.
- Areas are square metres unless stated otherwise.
- `centroid_e` and `centroid_n` use Slovenian D96/TM coordinates, EPSG:3794, in metres.
- `location` is a generated PostGIS point in EPSG:3794. It is null when either source centroid coordinate is missing.
- Empty source values and known zero-date sentinels such as `00.00.0000` are stored as SQL `NULL`.
- Negative historical sales `transaction_id` values are valid source identifiers and must not be converted to positive numbers.
- Numeric categorical fields are decoded through `codebook_values` when the relevant source codebook is available.
- National transaction rows do not directly contain an administrative municipality code. Location belongs to associated property rows.

## Relationships

```text
source_files
├── transactions
│   ├── building_parts
│   └── land_parcels
├── rental_transactions
│   └── rental_building_parts
└── codebook_values / codebook_imports

municipalities                  ← optional administrative reference
cadastral_municipalities       ← building/parcel cadastral reference
```

## `source_files`

One record per unique imported ZIP archive. This is the provenance and resume ledger.

| Column | Type | Nullable | Meaning |
|---|---|---:|---|
| `id` | `bigint` | no | Internal generated source identifier. |
| `file_name` | `text` | no | Original ZIP filename. |
| `municipality_code` | `integer` | yes | Municipality filter for legacy municipality-scoped archives; null for national archives. |
| `dataset_year` | `integer` | yes | Source dataset year. |
| `dataset_type` | `text` | no | `sales` for BUY/KPP or `rent` for RENT/NP. |
| `dataset_scope` | `text` | no | `national` or legacy `municipality`. |
| `sha256` | `char(64)` | no | Unique SHA-256 checksum of the ZIP contents. |
| `downloaded_at` | `timestamptz` | yes | Local file modification/download time. |
| `imported_at` | `timestamptz` | no | Time PostgreSQL registered the archive. |

## `municipalities`

Current administrative municipalities from the JGP `municipalities` endpoint.

| Column | Source | Type | Meaning |
|---|---|---|---|
| `code` | `sifra` | `integer` | Official administrative municipality code. |
| `name` | `name` | `text` | Municipality name. |

## `cadastral_municipalities`

Current and historical cadastral municipalities. Historical codes found only in archives are retained.

| Column | Source | Type | Meaning |
|---|---|---|---|
| `code` | `sifko` / `SIFRA_KO` | `integer` | Cadastral municipality code. |
| `name` | `name` / `IME_KO` | `text` | Cadastral municipality name; it may be empty for incomplete historical records. |

## `transactions` — BUY/KPP agreements

One purchase transaction. Join property records with `transaction_id`.

| Column | Source CSV field | Type | Nullable | Meaning / unit |
|---|---|---|---:|---|
| `transaction_id` | `ID_POSLA` | `bigint` | no | GURS transaction identifier. Negative values are valid for legacy 2007–2014 records. |
| `transaction_type` | `VRSTA_KUPOPRODAJNEGA_POSLA` | `integer` | yes | Purchase transaction type code; decode through the source codebook. |
| `effective_date` | `DATUM_UVELJAVITVE` | `date` | yes | Date the transaction version became effective. |
| `contract_date` | `DATUM_SKLENITVE_POGODBE` | `date` | yes | Contract signing date. |
| `contract_price` | `POGODBENA_CENA_ODSKODNINA` | `numeric(16,2)` | yes | Total contractual price/compensation, normally EUR. |
| `vat_included` | `VKLJUCENOST_DDV` | `boolean` | yes | Whether VAT is included. |
| `vat_rate` | `STOPNJA_DDV` | `numeric(8,4)` | yes | VAT rate supplied by GURS. |
| `lease_expiry_date` | `DATUM_IZTEKA_LIZINGA` | `date` | yes | Leasing expiry date when the purchase is related to leasing. |
| `lease_termination_date` | `DATUM_PRENEHANJA_LIZINGA` | `date` | yes | Leasing termination date. |
| `notes` | `OPOMBE_O_PRAVNEM_POSLU` | `text` | yes | Source notes about the legal transaction. |
| `agency_brokered` | `POSREDOVANJE_NEPREMICNINSKE_AGENCIJE` | `boolean` | yes | Whether a real-estate agency brokered the transaction. |
| `last_changed_date` | `DATUM_ZADNJE_SPREMEMBE_POSLA` | `date` | yes | Last source modification date. |
| `last_effective_date` | `DATUM_ZADNJE_UVELJAVITVE_POSLA` | `date` | yes | Last effective-version date. |
| `deed_type` | `VRSTA_AKTA` | `integer` | yes | Legal deed type code. |
| `marketability` | `TRZNOST_POSLA` | `integer` | yes | Market transaction classification code. |
| `dataset_year` | `LETO` | `integer` | no | GURS dataset/filter year. |
| `municipality_code` | `RPE_OBCINE_SIFRA` | `integer` | yes | Legacy municipality-scoped value; national archives omit it. Use property rows for location. |
| `source_file_id` | derived | `bigint` | no | FK to `source_files`. |

## `building_parts` — BUY/KPP building units

Every source row is preserved. A transaction may contain several rows with the same cadastral identifiers but different attributes.

| Column | Source CSV field | Type | Nullable | Meaning / unit |
|---|---|---|---:|---|
| `id` | derived | `bigint` | no | Internal generated row identifier. |
| `transaction_id` | `ID_POSLA` | `bigint` | no | FK to `transactions`. |
| `cadastral_municipality_code` | `SIFRA_KO` | `integer` | yes | Cadastral municipality code. |
| `building_number` | `STEVILKA_STAVBE` | `text` | yes | Cadastral building number. |
| `building_part_number` | `STEVILKA_DELA_STAVBE` | `text` | yes | Cadastral building-part number; absent for some unregistered historical records. |
| `cadastral_municipality_name` | `IME_KO` | `text` | yes | Cadastral municipality name. |
| `municipality_name` | `OBCINA` | `text` | yes | Administrative municipality name. |
| `parcel_number_for_geolocation` | `PARCELNA_STEVILKA_ZA_GEOLOKACIJO` | `text` | yes | Parcel used to locate the building. |
| `internal_building_part_label` | `INTERNA_OZNAKA_DELA_STAVBE` | `text` | yes | Source internal building-part label. |
| `recorded_status` | `EVIDENTIRANOST_DELA_STAVBE` | `integer` | yes | Registration/evidencing status code. |
| `settlement` | `NASELJE` | `text` | yes | Settlement name. |
| `street` | `ULICA` | `text` | yes | Street name. |
| `house_number` | `HISNA_STEVILKA` | `text` | yes | House number. |
| `house_number_suffix` | `DODATEK_HS` | `text` | yes | House-number suffix. |
| `apartment_or_business_unit_number` | `STEVILKA_STANOVANJA_ALI_POSLOVNEGA_PROSTORA` | `text` | yes | Apartment/business-unit number. |
| `building_part_type` | `VRSTA_DELA_STAVBE` | `integer` | yes | Sold building-part type code. |
| `construction_year` | `LETO_IZGRADNJE_DELA_STAVBE` | `integer` | yes | Construction year. |
| `building_completed` | `STAVBA_JE_DOKONCANA` | `boolean` | yes | Whether construction is complete. |
| `construction_phase` | `GRADBENA_FAZA` | `integer` | yes | Construction phase code. |
| `new_construction` | `NOVOGRADNJA` | `boolean` | yes | Whether classified as new construction. |
| `sold_area` | `PRODANA_POVRSINA` | `numeric(14,2)` | yes | Sold area, m². |
| `sold_share` | `PRODANI_DELEZ_DELA_STAVBE` | `text` | yes | Sold fractional share, kept as source text such as `1/2` or `NP`. |
| `sold_building_part_area` | `PRODANA_POVRSINA_DELA_STAVBE` | `numeric(14,2)` | yes | Sold building-part area, m². |
| `sold_usable_area` | `PRODANA_UPORABNA_POVRSINA_DELA_STAVBE` | `numeric(14,2)` | yes | Sold usable area, m². |
| `floor` | `NADSTROPJE_DELA_STAVBE` | `text` | yes | Floor description/code as supplied. |
| `outdoor_parking_spaces` | `STEVILO_ZUNANJIH_PARKIRNIH_MEST` | `integer` | yes | Number of outdoor parking spaces. |
| `atrium` | `ATRIJ` | `boolean` | yes | Whether an atrium is included. |
| `atrium_area` | `POVRSINA_ATRIJA` | `numeric(14,2)` | yes | Atrium area, m². |
| `property_notes` | `OPOMBE_O_NEPREMICNINI` | `text` | yes | Source property notes. |
| `actual_use` | `DEJANSKA_RABA_DELA_STAVBE` | `text` | yes | Actual-use code and description as supplied. |
| `position_in_building` | `LEGA_DELA_STAVBE_V_STAVBI` | `text` | yes | Position/floor location within the building. |
| `room_count` | `STEVILO_SOB` | `numeric(8,2)` | yes | Number of rooms. |
| `building_part_area` | `POVRSINA_DELA_STAVBE` | `numeric(14,2)` | yes | Registered building-part area, m². |
| `usable_area` | `UPORABNA_POVRSINA` | `numeric(14,2)` | yes | Usable area, m². |
| `rooms` | `PROSTORI_DELA_STAVBE` | `text` | yes | Source list/description of spaces. |
| `contractual_price` | `POGODBENA_CENA_DELA_STAVBE` | `numeric(16,2)` | yes | Price attributed to this building part, normally EUR. |
| `vat_rate` | `STOPNJA_DDV_DELA_STAVBE` | `numeric(8,4)` | yes | VAT rate for this building part. |
| `centroid_e` | `E_CENTROID` | `numeric(12,2)` | yes | D96/TM easting (X), EPSG:3794, metres. |
| `centroid_n` | `N_CENTROID` | `numeric(12,2)` | yes | D96/TM northing (Y), EPSG:3794, metres. |
| `location` | generated | `geometry(Point,3794)` | yes | PostGIS point generated from `centroid_e` and `centroid_n`; GiST-indexed. |
| `dataset_year` | `LETO` | `integer` | no | Source dataset year. |
| `municipality_code` | `RPE_OBCINE_SIFRA` | `integer` | yes | Legacy municipality archive code; national files omit it. |
| `source_file_id` | derived | `bigint` | no | FK to `source_files`. |
| `source_row_number` | derived | `integer` | no | Original CSV line number, including the header as line 1. |

## `land_parcels` — BUY/KPP land

| Column | Source CSV field | Type | Nullable | Meaning / unit |
|---|---|---|---:|---|
| `id` | derived | `bigint` | no | Internal generated row identifier. |
| `transaction_id` | `ID_POSLA` | `bigint` | no | FK to `transactions`. |
| `cadastral_municipality_code` | `SIFRA_KO` | `integer` | yes | Cadastral municipality code; may be absent in incomplete historical rows. |
| `parcel_number` | `PARCELNA_STEVILKA` | `text` | yes | Parcel number. |
| `cadastral_municipality_name` | `IME_KO` | `text` | yes | Cadastral municipality name. |
| `municipality_name` | `OBCINA` | `text` | yes | Administrative municipality name. |
| `land_type` | `VRSTA_ZEMLJISCA` | `integer` | yes | Land type code; decode through the source codebook. |
| `permanent_crop_type` | `VRSTA_TRAJNEGA_NASADA` | `integer` | yes | Permanent crop/plantation type code. |
| `permanent_crop_age` | `STAROST_TRAJNEGA_NASADA` | `integer` | yes | Permanent crop age, normally years. |
| `sold_share` | `PRODANI_DELEZ_PARCELE` | `text` | yes | Sold fractional parcel share as source text. |
| `property_notes` | `OPOMBE_O_NEPREMICNINI` | `text` | yes | Source property notes. |
| `parcel_area` | `POVRSINA_PARCELE` | `numeric(14,2)` | yes | Parcel area, m². |
| `contractual_price` | `POGODBENA_CENA_PARCELE` | `numeric(16,2)` | yes | Price attributed to this parcel, normally EUR. |
| `vat_rate` | `STOPNJA_DDV_PARCELE` | `numeric(8,4)` | yes | VAT rate for this parcel. |
| `centroid_e` | `E_CENTROID` | `numeric(12,2)` | yes | D96/TM easting (X), EPSG:3794, metres. |
| `centroid_n` | `N_CENTROID` | `numeric(12,2)` | yes | D96/TM northing (Y), EPSG:3794, metres. |
| `location` | generated | `geometry(Point,3794)` | yes | PostGIS point generated from `centroid_e` and `centroid_n`; GiST-indexed. |
| `dataset_year` | `LETO` | `integer` | no | Source dataset year. |
| `municipality_code` | `RPE_OBCINE_SIFRA` | `integer` | yes | Legacy municipality archive code; national files omit it. |
| `source_file_id` | derived | `bigint` | no | FK to `source_files`. |
| `source_row_number` | derived | `integer` | no | Original CSV line number. |

## `rental_transactions` — RENT/NP agreements

| Column | Source CSV field | Type | Nullable | Meaning / unit |
|---|---|---|---:|---|
| `transaction_id` | `ID_POSLA` | `bigint` | no | GURS rental transaction identifier. |
| `rental_transaction_type` | `VRSTA_NAJEMNEGA_POSLA` | `integer` | yes | Rental transaction type code. |
| `effective_date` | `DATUM_UVELJAVITVE` | `date` | yes | Effective-version date. |
| `contract_date` | `DATUM_SKLENITVE_POGODBE` | `date` | yes | Contract signing date. |
| `contract_rent` | `POGODBENA_NAJEMNINA` | `numeric(16,2)` | yes | Total contractual rent, normally EUR for the source period. |
| `operating_costs_included` | `VKLJUCENOST_OBRATOVALNIH_STROSKOV_V_NAJEMNINO` | `boolean` | yes | Whether operating costs are included. |
| `vat_included` | `VKLJUCENOST_DDV` | `boolean` | yes | Whether VAT is included. |
| `vat_rate` | `STOPNJA_DDV` | `numeric(8,4)` | yes | VAT rate. |
| `rental_start_date` | `DATUM_ZACETKA_NAJEMA` | `date` | yes | Rental start date; zero-date sentinels become null. |
| `rental_end_date` | `DATUM_PRENEHANJA_NAJEMA` | `date` | yes | Rental termination date. |
| `rental_time` | `CAS_NAJEMA` | `text` | yes | Source rental-time classification. |
| `rental_duration` | `TRAJANJE_NAJEMA` | `text` | yes | Source rental duration value/description. |
| `closure_or_early_termination_date` | `DATUM_ZAKLJUCKA_NAJEMA_DATUM_PREDCASNE_PREKINITVE_NAJEMA` | `date` | yes | Completion or early termination date. |
| `notes` | `OPOMBE_O_PRAVNEM_POSLU` | `text` | yes | Source legal-transaction notes. |
| `agency_brokered` | `POSREDOVANJE_NEPREMICNINSKE_AGENCIJE` | `boolean` | yes | Whether an agency brokered the rental. |
| `last_changed_date` | `DATUM_ZADNJE_SPREMEMBE_POSLA` | `date` | yes | Last source modification date. |
| `last_effective_date` | `DATUM_ZADNJE_UVELJAVITVE_POSLA` | `date` | yes | Last effective-version date. |
| `deed_type` | `VRSTA_AKTA` | `integer` | yes | Legal deed type code. |
| `marketability` | `TRZNOST_POSLA` | `integer` | yes | Market transaction classification code. |
| `dataset_year` | `LETO` | `integer` | no | Source dataset year. |
| `source_file_id` | derived | `bigint` | no | FK to `source_files`. |

## `rental_building_parts` — RENT/NP rented spaces

| Column | Source CSV field | Type | Nullable | Meaning / unit |
|---|---|---|---:|---|
| `id` | derived | `bigint` | no | Internal generated row identifier. |
| `transaction_id` | `ID_POSLA` | `bigint` | no | FK to `rental_transactions`. |
| `cadastral_municipality_code` | `SIFRA_KO` | `integer` | yes | Cadastral municipality code. |
| `cadastral_municipality_name` | `IME_KO` | `text` | yes | Cadastral municipality name. |
| `municipality_name` | `OBCINA` | `text` | yes | Administrative municipality name. |
| `building_number` | `STEVILKA_STAVBE` | `text` | yes | Cadastral building number. |
| `building_part_number` | `STEVILKA_DELA_STAVBE` | `text` | yes | Cadastral building-part number. |
| `internal_rented_spaces_label` | `INTERNA_OZNAKA_ODDANIH_PROSTOROV` | `text` | yes | Source internal rented-space label. |
| `settlement` | `NASELJE` | `text` | yes | Settlement. |
| `street` | `ULICA` | `text` | yes | Street. |
| `house_number` | `HISNA_STEVILKA` | `text` | yes | House number. |
| `house_number_suffix` | `DODATEK_HS` | `text` | yes | House-number suffix. |
| `apartment_or_business_unit_number` | `STEVILKA_STANOVANJA_ALI_POSLOVNEGA_PROSTORA` | `text` | yes | Apartment/business-unit number. |
| `rented_spaces_type` | `VRSTA_ODDANIH_PROSTOROV` | `integer` | yes | Rented-space type code. |
| `furnishing` | `OPREMLJENOST_ODDANIH_PROSTOROV` | `integer` | yes | Furnishing/equipment code. |
| `microlocation` | `MIKROLOKACIJA_ODDANIH_PROSTOROV` | `integer` | yes | Microlocation code. |
| `storefront` | `IZLOZBA` | `boolean` | yes | Whether the space has a storefront/display window. |
| `shopping_center` | `NAKUPOVALNI_CENTER` | `boolean` | yes | Whether the space is in a shopping centre. |
| `rented_spaces_area` | `POVRSINA_ODDANIH_PROSTOROV` | `numeric(14,2)` | yes | Rented area, m². |
| `rented_area_equals_building_part` | `POVRSINA_ODDANIH_PROSTOROV_ENAKA_DELU_STAVBE` | `boolean` | yes | Whether rented area equals the entire building part. |
| `contractual_rent` | `POGODBENA_NAJEMNINA_POSAMEZNIH_ODDANIH_PROSTOROV` | `numeric(16,2)` | yes | Rent attributed to this rented space, normally EUR. |
| `rented_spaces_notes` | `OPOMBE_O_ODDANIH_PROSTORIH` | `text` | yes | Source notes about rented spaces. |
| `construction_year` | `LETO_IZGRADNJE_STAVBE` | `integer` | yes | Building construction year. |
| `actual_use` | `DEJANSKA_RABA_DELA_STAVBE` | `text` | yes | Actual-use code and description. |
| `position_in_building` | `LEGA_DELA_STAVBE_V_STAVBI` | `text` | yes | Position/floor within the building. |
| `room_count` | `STEVILO_SOB` | `numeric(8,2)` | yes | Number of rooms. |
| `building_part_area` | `POVRSINA_DELA_STAVBE` | `numeric(14,2)` | yes | Registered building-part area, m². |
| `usable_building_part_area` | `UPORABNA_POVRSINA_DELA_STAVBE` | `numeric(14,2)` | yes | Usable building-part area, m². |
| `rooms` | `PROSTORI_DELA_STAVBE` | `text` | yes | Source list/description of spaces. |
| `rented_usable_area` | `UPORABNA_POVRSINA_ODDANIH_PROSTOROV` | `numeric(14,2)` | yes | Usable area of rented spaces, m². |
| `hospitality_garden` | `GOSTINSKI_VRT` | `boolean` | yes | Whether a hospitality garden is included. |
| `garden_included_in_rent` | `VKLJUCENOST_GOSTINSKEGA_VRTA_V_NAJEMNINO` | `boolean` | yes | Whether the garden is included in rent. |
| `hospitality_garden_area` | `POVRSINA_GOSTINSKEGA_VRTA` | `numeric(14,2)` | yes | Hospitality garden area, m². |
| `centroid_e` | `E_CENTROID` | `numeric(12,2)` | yes | D96/TM easting (X), EPSG:3794, metres. |
| `centroid_n` | `N_CENTROID` | `numeric(12,2)` | yes | D96/TM northing (Y), EPSG:3794, metres. |
| `location` | generated | `geometry(Point,3794)` | yes | PostGIS point generated from `centroid_e` and `centroid_n`; GiST-indexed. |
| `dataset_year` | `LETO` | `integer` | no | Source dataset year. |
| `source_file_id` | derived | `bigint` | no | FK to `source_files`. |
| `source_row_number` | derived | `integer` | no | Original CSV line number. |

## `codebook_values`

Categorical source values from `sifranti` CSV files.

| Column | Source | Type | Meaning |
|---|---|---|---|
| `codebook_id` | `ID` | `integer` | Source codebook identifier. |
| `codebook_name` | `SIFRANT` | `text` | Slovenian codebook/category name. |
| `numeric_value` | `NUMERICNA_VREDNOST` | `integer` | Numeric categorical value stored in source records. |
| `description` | `OPIS` | `text` | Slovenian value description. |
| `source_file_id` | derived | `bigint` | Archive that first introduced the tracked codebook version. |

Lookup example:

```sql
SELECT lp.*, cb.description AS land_type_description
FROM land_parcels lp
LEFT JOIN codebook_values cb
  ON cb.numeric_value = lp.land_type
 AND cb.codebook_name = 'Vrsta zemljišča';
```

## `codebook_imports`

Tracks unique codebook file versions so identical codebooks are not repeatedly upserted.

| Column | Type | Meaning |
|---|---|---|
| `checksum` | `char(64)` | SHA-256 of the codebook CSV payload. |
| `source_file_id` | `bigint` | Archive in which this version was first imported. |
| `imported_at` | `timestamptz` | Import timestamp. |

## Coordinate usage

## Analytical property classification

`analytics.building_part_type_groups` maps the 15 GURS `Vrsta dela
stavbe` codes to stable API groups. `analytics.classify_sale_package(...)`
classifies all components belonging to one transaction, rather than treating
each row as a separate sale.

One primary use wins when its auxiliary components are land, a garage, or a
parking space. For example, a house plus its garden parcel and garage remains
`house`; an apartment plus parking remains `apartment`. Packages containing
different primary uses, such as a house and an apartment, are `mixed`.
Land-only and garage-only packages remain `land` and `garage`; unrecognized
or empty packages are `unknown`.

## `analytics.sale_facts`

One persisted analytical row per sale transaction, rebuilt with
`SELECT analytics.refresh_sale_facts()`. PostgreSQL/PostGIS performs the
aggregation and spatial assignment; the API does not load the source dataset
into memory.

The table records package component counts, stable property group, comparable
area, analytical price, price/m², quality reasons, representative point, and
current statistical-region, municipality, district, community, and settlement
IDs. `price_derivation` is `component_prices` only when every component has a
positive attributed price, `single_primary_transaction_price` when one primary
property safely uses the transaction total, and `unavailable` otherwise.

`analytics.sale_outlier_evaluations` stores the versioned IQR/MAD decision,
cohort level and size, robust bounds, modified z-score, and reasons for every
sale fact. Cohorts prefer the same building, property group, 20 m² area band,
and contract year (minimum 5), then municipality/year (minimum 15), then
statistical region/year (minimum 30). Conservative dispersion floors prevent
rounded or repeated source values from creating near-zero bounds.

`representative_eligible` excludes only deterministic data-quality failures and
detected statistical outliers from cleaned means/trends. Nothing is deleted;
raw values and evaluations remain queryable. Implausibly tiny/large areas are
classified as data-quality issues before market-outlier evaluation.

## `analytics.rental_facts`

One row per rental agreement, refreshed with
`analytics.refresh_rental_facts()`. Rent amounts are monthly. Heterogeneous
packages such as an apartment plus garage are retained as `mixed` but excluded
when their total rent cannot be safely separated. Homogeneous packages may use
the transaction total; complete component rents take precedence.

`monthly_rent_reported` preserves the attributed source amount.
`monthly_rent_ex_vat` removes a known GURS VAT code when VAT is included, and
`monthly_rent_per_sqm` divides that normalized amount by rented area. The fact
also records fixed/indefinite term, duration in months, operating-cost
inclusion, market/non-market type, building identity, point, geographic IDs,
derivation, and all quality reasons.

Representative rental facts require a market transaction, homogeneous known
property group, valid date, area, monthly rent, VAT interpretation, and
duration. Apartment rules follow the published GURS methodology: 10–300 m²,
EUR 10–10,000 monthly rent, and EUR 1–50/m². Other property groups use
documented conservative plausibility bounds pending group-specific calibration.

Centroids are projected points, not longitude/latitude. Convert the generated PostGIS point from EPSG:3794 to WGS84/EPSG:4326 with:

```sql
SELECT
    transaction_id,
    parcel_number,
    ST_Transform(location, 4326) AS gps_location
FROM land_parcels
WHERE location IS NOT NULL;
```

Use `ST_X(ST_Transform(location, 4326))` for longitude and `ST_Y(...)` for latitude.

## Analysis cautions

1. `dataset_year` is the source extraction/filter year; use `contract_date` for contract-time analysis when populated.
2. A transaction can contain multiple property rows. Summing `contract_price` after joining to property tables duplicates the transaction price unless grouped by `transaction_id` first.
3. `contractual_price` on an individual property is often null even when total `contract_price` exists.
4. Fractional shares are source text and can contain values such as `NP`; they require domain-aware parsing before numeric analysis.
5. Centroids are approximate location points and should not be treated as legal parcel/building geometry.
6. Missing historical identifiers are retained intentionally; do not remove rows solely because cadastral fields are null.

## API analytics schema

The Go API reads quality-controlled, derived data from the `analytics` schema. Source imports remain in `public`; the API must not mutate them.

### `analytics.geographic_units`

Stores current GURS RPE boundary units used by national, statistical-region,
municipality, city-district, local-community, and settlement analytics.
`real-estate-etl sync-geographies` imports and updates these boundaries.

| Column | Type | Meaning |
|---|---|---|
| `id` | `bigint` | Internal generated geographic-unit identifier used by API routes and hierarchy joins. |
| `level` | `text` | Controlled hierarchy level: country, statistical region, municipality, city district, local community, or settlement. |
| `source_collection` | `text` | GURS RPE OGC collection identifier. |
| `gurs_eid` | `text` | Stable source EID; unique within its collection. |
| `code` | `integer` | Source unit code where available. |
| `composite_code` | `integer` | Source composite code including parent context where available. |
| `name` | `text` | Official source name. Names are not unique identifiers. |
| `parent_id` | `bigint` | Optional FK to the direct parent geographic unit. |
| `parent_gurs_eid` | `text` | Parent municipality EID supplied by GURS for sub-municipal units. |
| `source_updated_at` | `timestamptz` | GURS source modification timestamp. |
| `source_properties` | `jsonb` | Preserved source attributes not promoted to typed columns. |
| `geometry_wgs84` | `geometry(MultiPolygon,4326)` | Source/web-map boundary in longitude/latitude. |
| `geometry_d96tm` | `geometry(MultiPolygon,3794)` | Generated boundary used for joins with property centroids. |
| `imported_at` | `timestamptz` | Time this database version was imported. |

Both geometry columns have GiST indexes. Level, parent, and normalized-name lookups have B-tree indexes.

### `analytics.refresh_runs`

Audit trail for future boundary and analytical refresh processes.

| Column | Type | Meaning |
|---|---|---|
| `id` | `bigint` | Internal generated run identifier. |
| `process` | `text` | Refresh process name. |
| `status` | `text` | `running`, `succeeded`, or `failed`. |
| `source_version` | `text` | Optional source release/checksum/version. |
| `started_at` | `timestamptz` | Refresh start time. |
| `finished_at` | `timestamptz` | Completion time; null only while running. |
| `details` | `jsonb` | Counts, timing, diagnostics, or failure metadata. |

### `analytics.rental_outlier_evaluations`

Auditable robust evaluations of rental price per m². Comparable cohorts use
property group, 20 m² area bands, contract year, and municipality (minimum 15)
with statistical-region fallback (minimum 30). `rental-outlier-v1` combines IQR
and approximate MAD rules. Flagged rows remain in source and fact tables;
`analytics.rental_facts.representative_eligible` controls cleaned aggregates.

### `analytics.anomaly_signals`

One non-destructive signal record per sale or rental fact. It stores named
flags plus normalized hard-rule, robust-cohort, model-residual, Isolation
Forest, duplicate, spatial, and combined review scores. Model fields remain
null until `real-estate-etl refresh-models` runs. Human review status, notes,
reviewer, and time are preserved when signals are refreshed.

### Model and index tables

- `analytics.model_runs`: immutable model version, feature schema, random seed,
  training window/sample, and out-of-fold metrics.
- `analytics.valuation_predictions`: observed and predicted amount per m²,
  empirical lower/upper range, and residual for every model-eligible fact.
- `analytics.repeat_sale_pairs`: consecutive representative sales of the same
  cadastral building part, at least 180 days apart.
- `analytics.market_indices`: national repeat-sales and constant-quality
  hedonic series with method version, sample, base period, and model run.

### Identity and preferences

- `app_users`: normalized email, bcrypt password hash, display name, and
  lifecycle timestamps.
- `auth_sessions`: SHA-256 hashes of random opaque bearer tokens; raw tokens
  are never stored.
- `user_preferences`: one bounded JSON object per user.

The `real_estate_api` role remains unable to modify imported transactions or
analytical values. It has narrowly scoped writes for identity, sessions,
preferences, and anomaly review metadata only.
