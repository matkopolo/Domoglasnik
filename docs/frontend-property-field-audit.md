# Phase 2 frontend property-field audit

Date: 2026-07-29

## Policy

This is a public-field whitelist. A field is rendered only when meaningful,
interpretable, and populated. Nulls, blank strings, invalid numbers, source
sentinels, and generic labels are omitted rather than replaced by `Ni podatka`
or a fake zero.

`positionInBuilding` is real ETN source data from
`LEGA_DELA_STAVBE_V_STAVBI` via `position_in_building`; it is not hardcoded.
The generic value `nadstropje` is not a floor number. It is omitted from compact
views and shown in detail only when a more specific value adds information.

Availability percentages below were measured on the populated local database:
605,368 sale building-part rows, 1,055,799 land-parcel rows, and 506,317 rental
space rows. Fact-level fields use the existing quality/missingness profiles.

## Transaction and analytical fields

| Source / derivation | Database/API field | Semantics | API before | UI before | Availability | Public reliability | Target | Risk | Decision |
|---|---|---|---|---|---:|---|---|---|---|
| `ID_POSLA` | `transactionId` | ETN transaction key | yes | popup title | 100% | high | technical heading | low | Show |
| contract date | `contractDate` | legal/recorded contract date | yes | compact | high; quality flagged when absent | high when present | compact + detail | low | Show if present |
| source extraction | `datasetYear` | archive year, not contract year | yes | no | 100% | high but technical | methodology | low | Show as source vintage only |
| derived package | `propertyGroup` | comparable analytical group | yes | marker selection | 100% | high | heading | low | Show translated label |
| derived | `area` | comparable denominator | yes | compact | quality-dependent | high when eligible | compact + detail | low | Show with definition |
| derived | `price` / rent fields | analytical amount | yes | sale partial | quality-dependent | high with derivation | compact + detail | low | Show if present |
| derived | amount per m² | comparable metric | yes | yes | quality-dependent | high with warning | compact + detail | low | Show |
| derived | `priceDerivation` / `rentDerivation` | component or transaction attribution | yes | no | 100% | high | methodology | low | Translate and show |
| derived | `quality` | eligible/representative/reasons | yes | no | 100% | high | quality section/badge | low | Show |
| robust evaluation | `outlier` | cohort, bounds, z-score, rule | yes | no | where evaluated | high as warning, not accusation | quality section | low | Show |
| spatial assignment | `geographies` | region→municipality→local area | yes | no | coordinate-dependent | current-boundary limitation | breadcrumb | low | Show |
| model | valuation and interval | estimate, not official value | separate endpoint | no | model-eligible only | reliability must be visible | valuation section | low | Show with warnings |
| contextual cohort | benchmark/percentile/difference | same-year representative cohort | missing | no | sample-dependent | descriptive | comparison section | low | Add backend-derived |

## Sale building/property fields

| ETN source | Database/API field | Codebook/unit | API before | UI before | Availability | Public reliability | Target | Risk | Decision |
|---|---|---|---|---|---:|---|---|---|---|
| `VRSTA_DELA_STAVBE` | `buildingPartType` | `Vrsta dela stavbe` | raw code | no | near 100% | high with description | composition | low | Return code + Slovenian description |
| `LETO_IZGRADNJE` | `constructionYear` | year | yes | compact first part | 98.0% | high after range validation | detail/building | low | Show |
| `NADSTROPJE` | `floor` | source text | yes | fixed row with fallback | 20.5% | useful when meaningful | detail only | low | Omit missing/blank; never infer |
| `LEGA_DELA_STAVBE_V_STAVBI` | `positionInBuilding` | source text | yes | compact raw | 60.8% | generic values weak | detail only | low | Omit `nadstropje`; show specific value |
| apartment/business unit | `unitNumber` | source identifier | yes | no | not measured | sensitive household identifier | none | medium/high | Do not expose in public UI |
| room count | `roomCount` | count | yes | no | 26.5% | useful when non-negative | detail | low | Show |
| sold area | `soldArea` | m² | yes | extra-part summary | substantial | source-specific | composition | low | Show with label |
| sold usable area | `soldUsableArea` | m² | yes | no | partial | source-specific | composition | low | Show if present |
| registered part area | `buildingPartArea` | m² | yes | no | partial | not necessarily sold area | technical detail | low | Show clearly labelled |
| usable area | `usableArea` | m² | yes | fixed row | 73.3% | useful | detail and compact when primary | low | Show |
| outdoor parking | `outdoorParkingSpaces` | count | yes | no | partial | zero is meaningful | detail | low | Show including valid zero |
| component price | `contractualPrice` | EUR | yes | no | 0.1% | sparse but defensible | composition | low | Show only when present |
| settlement/street/house | address fields | text | yes | first-part address | street 72.5% | useful when present | heading | low | Compose from all meaningful parts |
| building-part number | `buildingPartNumber` | cadastral identifier | yes | no | high | technical; can identify unit | technical/private | medium | Do not display publicly |
| free-text notes | retained source columns | uncontrolled text | no | no | variable | unreviewed | none | high | Keep private |

## Land fields

| ETN source / derivation | Database/API field | Semantics | API before | UI before | Availability | Public reliability | Target | Risk | Decision |
|---|---|---|---|---|---:|---|---|---|---|
| `VRSTA_ZEMLJISCA` | `landType` | codebook 1–10 | raw code | no | 100.0% | high with description | land composition | low | Return code + description |
| `POVRSINA_PARCELE` | `parcelArea` | m² | yes | no | 96.2% | high when positive | land composition | low | Show and total |
| parcel number | `parcelNumber` | cadastral identifier | yes | no | 100.0% | technical, location-sensitive | technical section | medium | Do not show in compact; allow labelled technical detail only |
| component price | `contractualPrice` | EUR | yes | no | effectively 0.0% | too sparse | detail if present | low | Omit normally |
| Phase 1 derivation | readiness class / strict flag | verified ETN readiness | not in transaction DTO | no | land-only facts | high with planning caveat | land heading | low | Add |
| package composition | counts/mixed state | contract versus parcels | partial arrays | simplistic note | 100% derivable | high | land section | low | Show contracts/parcels distinctly |
| property notes | retained source text | uncontrolled | no | no | variable | unreviewed | none | high | Never expose |

## Rental fields

| ETN source | Database/API field | Codebook/unit | API before | UI before | Availability | Public reliability | Target | Risk | Decision |
|---|---|---|---|---|---:|---|---|---|---|
| reported rent | `monthlyRentReported` | EUR/month | yes | no | quality-dependent | useful with derivation | detail | low | Show |
| normalized rent | `monthlyRentExVat` | EUR/month | yes | no | quality-dependent | preferred comparison | compact + detail | low | Show |
| rent per m² | `monthlyRentPerSqm` | EUR/m²/month | yes | marker only | quality-dependent | high when eligible | compact + detail | low | Show |
| start/end | rental dates | dates | yes | no | partial | useful | detail | low | Show if present |
| derived term/duration | `rentalTerm`, `durationMonths` | fixed/indefinite, months | yes | no | derived | high | detail | low | Show |
| operating costs | `operatingCostsIncluded` | boolean | yes | no | partial | high when known | detail | low | Show true/false text |
| market rent | `isMarketRent` | quality classification | yes | no | 100% derived | high | quality badge | low | Show |
| rented-space type | `rentedSpaceType` | `Vrsta oddanih prostorov` | raw code | no | 100.0% | high with description | composition | low | Return code + description |
| furnishing | `furnishing` | raw code; no matching imported codebook found | raw code | no | 100.0% | uninterpretable without domain map | none | low | Defer display |
| rented area | `area` | m² | yes | no | 99.9% | high | composition | low | Show |
| rented usable area | `usableArea` | m² | yes | no | 9.5% | useful but sparse | detail | low | Show if present |
| construction year | `constructionYear` | year | yes | no | 99.9% | high | detail | low | Show |
| address | address fields | text | yes | no | street 88.0% | useful | heading | low | Show |
| microlocation/storefront/garden | retained source columns | domain-specific | no | no | not yet measured | requires interpretation audit | none | low | Defer with data/domain requirement |
| free-text notes | retained source text | uncontrolled | no | no | variable | unreviewed | none | high | Never expose |

## Central display-value rules

The implementation must use one tested helper for both DOM popups and React
detail panels:

- null, undefined, blank, whitespace, NaN, and sentinels are hidden;
- valid numeric zero remains visible;
- compact views exclude floor and position entirely;
- detailed floor appears only when meaningful;
- detailed position excludes generic `nadstropje`;
- absent price/rent is not formatted as zero;
- multiple parts/parcels are rendered as a composition;
- unknown codebook values remain diagnostic metadata but are not shown as raw
  numbers;
- source notes, unit numbers, building-part identifiers, and reviewer identity
  are outside the public whitelist.

