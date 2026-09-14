# Property history and resale scenarios

Implemented against baseline `f14f25a2175a3d570c988c37401f8030c151d33e`.
This feature supplements valuation training, strict land populations, outlier
classification and the published repeat-sales index; it changes none of them.

## Navigation and source information

Select a sale and open **Zgodovina**. History loads only when opened and spans
all available years, not just the dashboard period. Events open transactions
through the existing map/selection flow. Rental agreements are not purchases;
their existing history remains unchanged.

**Čas med ponovnimi prodajami** is an on-demand area summary. For a selected
building/transaction it uses its municipality, not just the building, like the
recent-transactions section. It respects the selected sale population filters.

**Odpri lokacijo v Google Zemljevidih** uses existing transaction/building
latitude and longitude. It requires no API key and does not replace MapLibre.
Google receives the coordinates only when the external link is clicked. Missing
or invalid coordinates produce no link; the UI warns that package centroids
and ETN locations are approximate, not necessarily entrances or parcel outlines.

## Identity and evidence

| Evidence | Matching key | Meaning |
| --- | --- | --- |
| Building part | cadastral municipality + building + part number | Same recorded component identifier |
| Parcel | cadastral municipality + parcel number | Same recorded parcel identifier |
| Building/parcel reference | cadastral municipality + `parcel_number_for_geolocation` | Weaker geolocation evidence only |

Positive cadastral codes and positive numeric building/part numbers are required.
Parcel identifiers accept a positive number and optional positive `/subnumber`.
Unsupported formats and missing/placeholder identifiers are not guessed; genuine
unusual historical identifiers may be excluded. No fuzzy address or proximity
match establishes identity. Splits, mergers, renumbering, shares, inheritance,
gifts and missing transactions can interrupt continuity. Identical recorded
identifiers do not prove uninterrupted ownership or unchanged physical identity.

Events retain nonrepresentative and possible-duplicate records, with source
type, share, component area, reported contract total, component price when present,
construction year and package count. Contract totals are not allocated to parts.
Migration 039 uses `transactions.contract_price`, not derived analytical prices.

Only IDs present in published `analytics.sale_facts` participate. Source values
come from current imported source tables, like existing transaction details;
they are not immutable source snapshots. Corrected source fields for an already
published ID can precede its next analytical release. History uses repeatable-read
for a consistent individual response, not a new source-versioning guarantee.

## Resale methodology

Exact identities are collapsed to one event per contract, then ordered by date
and transaction ID. Missing dates sort last. Repeated component rows cannot
become extra sales. Each pair uses the immediately preceding recorded contract;
invalid events are not skipped to manufacture a clean pair.

Intervals shorter than 180 days remain visible. This does not change the separate
repeat-sale index's 180-day cutoff. Zero-day ties do not enter return statistics.

Price comparisons and published area interval summaries require both events to:

- be quality eligible and representative, with no possible-duplicate flags;
- have one component in the agreement and one row for the identity;
- have a known whole share (`1`, `1/1`, `1.0`, `100/100`, ignoring spaces);
- have positive reported contract prices and component areas;
- have identical non-null source types and area within 1% of the earlier area;
- have strictly increasing dates.

Thus apartment-plus-parking packages remain visible but do not receive a
component return. Unknown shares are not assumed complete. Renovation and other
unobserved changes remain possible even among eligible pairs.

`days = later date − earlier date`

`nominal difference = later reported price − earlier reported price`

`percentage difference = 100 × (later price / earlier price − 1)`

Area/date/property/land-scope filters apply to the **later** transaction; its
predecessor can predate the selected period. Strict land remains the default.
Pair count and distinct asset count are returned; P25/median/P75 days are withheld
below 30 pairs. Multiple pairs can belong to one asset, so these are descriptive
pair-weighted statistics, not independent-owner confidence intervals. The sample
is **observed comparable resales**, not all owners; never-resold properties are absent.

## Land changes and development evidence

Parcel code 7/8 followed by 1/2 yields **Sprememba evidentirane vrste zemljišča**.
Code 3 remains distinct, not verified buildable. Conflicting classifications in
one identity/contract are unknown rather than the minimum code. This flag does
not depend on price comparability and is not used as a valuation input.

ETN cannot prove legal rezoning, a permit or the change date. See
[municipal spatial acts](https://www.gov.si/teme/obcinski-prostorski-akti/).

**Možna poznejša pozidava** requires an earlier land sale and a later house
agreement with the same geolocation parcel reference and construction year
strictly after the land-sale year, no later than the house-sale year. This is
explicitly not proof of construction on, or ownership of, that parcel. Same-year
development is not inferred from a year-only field. No proximity-only house
match or confirmed agricultural → permit → house chain is produced.

## Inflation

The existing inflation graph uses SURS H281S `INDEKS=2` (year-over-year rates).
The new factor endpoint separately requests `INDEKS=1` (month/previous month)
from [SURS metadata](https://pxweb.stat.si/SiStatData/api/v1/en/Data/H281S.px).
It multiplies ratios after acquisition month A through resale month B. Both
endpoint months and every intervening month must exist; same published month
has factor 1. Missing data and upstream errors return unavailable, not zero.

`earlier price in later-month euros = earlier price × factor`

`real difference = later price − earlier price × factor`

`real percentage = 100 × (later price / (earlier price × factor) − 1)`

These are approximations from rounded monthly ratios, not daily indices or net
profits. No overlapping annual rates are compounded. Source/months/factor are
shown. The API caches the ratio series for six hours independently of its annual
series. Data is fetched only when the inflation disclosure is opened.

## Optional tax/cost scenario

The browser-only scenario is educational. It neither stores nor sends entered
financial data to the API, and never infers a previous seller's actual profit.
Version `SI-private-sale-2026-reviewed-2026-09-10` supports private individual
sales without VAT in 2026 only. Earlier/later disposal regimes and acquisitions
before 2002 are unsupported, not extrapolated.

The user enters dates, purchase/sale amounts, other cash costs, a **confirmed
taxable gain after applicable deductions**, DPN payer/exemption, and optional
confirmed main-home exemption. Taxable gain is deliberately not equated with
sale minus purchase minus arbitrary cash expenses. Eligible normative expenses,
documented investment and deductible taxes must be checked separately.

Current taxable-gain rates: under five completed years 25%; five–nine 20%;
ten–fourteen 15%; fifteen or more zero. Main-home exemption requires confirmation
of its conditions and at least three years of ownership; residence/use is not
inferred from ETN. Seller-paid DPN is calculated once at 2% of the assumed
applicable sale value. Buyer-paid or confirmed exempt DPN contributes zero to
the seller scenario. VAT cases are excluded.

[FURS purchase/sale guidance](https://www.fu.gov.si/zivljenjski_dogodki_prebivalci/nakup_in_prodaja_nepremicnine)
and [DPN guidance](https://www.fu.gov.si/davki_in_druge_dajatve/podrocja/davek_na_promet_nepremicnin)
were checked on 10 September 2026. A qualified tax review remains required
before commercial use; the calculator does not reproduce a FURS assessment.

`scenario difference = sale − purchase − entered other cash costs − seller DPN − capital-gains tax`

Tax amounts are rounded to cents. No inflation adjustment, financing principal
deduction or unentered expenses are silently added. The rule window must be
updated deliberately rather than extended automatically into 2027.

## Deployment, limits and verification

Migrations 038/039 add three B-tree indexes and two SELECT-only views for the API.
039 follows 038 instead of changing its already locally applied checksum. No
source rows are deleted; no facts, outliers, models or indices need rebuilding.

```bash
make db-migrate
docker compose up -d --build --no-deps --wait real-estate-api web
```

Do not reset data or restart the worker. Normal fresh setup includes migrations.

History is capped at 50 identities and 200 events per identity, with explicit
truncation and no pair calculations for a truncated identity. The area summary
is on demand and heavier. It disables parallel PostgreSQL workers **only inside
its transaction** to avoid Docker shared-memory exhaustion. No global memory
configuration or statistical values are changed.

Tests cover identity isolation, quick sales, shares/packages/duplicates, ambiguous
codes, source versus derived prices, geolocation references, suppression at 30,
date/area filters, API errors, CPI chaining/cache, tax boundaries and assumptions,
lazy loading, navigation and stale-response protection. Fresh-schema, checksum,
retry and least-privilege tests use disposable databases.

Live checks: transaction 959929 returned two component histories, each containing
three events; package totals did not become component returns. National apartment
resales in 2020–2026 returned 7,562 pairs / 6,999 assets in about 6.7 seconds on
the checked dataset; individual history took about 6–26 ms. These are observations,
not guarantees. SURS January 2024–January 2025 returned factor 1.0210628601 from
twelve monthly ratios.

Browser automation failed to initialize (`Cannot redefine property: process`);
visual QA is not claimed. Interaction tests and production builds provide the
available verification. Official parcel lineage, legal zoning/permit history,
actual ownership/holding periods, actual profits, automatic tax-base calculation
and full historical tax regimes remain outside the available evidence.

Verification commands executed:

- `make lint`: pipeline Ruff, Go vet, web ESLint and TypeScript passed.
- `make test`: pipeline (86), Go race/integration tests and web tests passed;
  subsequent targeted additions were rerun with `make api-test` and
  `make web-test` (106 frontend tests).
- `make web-build` and `make api-build`: production images built successfully.
- `MIGRATION_TEST_DATABASE_URL=... .venv/bin/pytest database/tests/test_production_migrate.py -q`:
  all three checks passed, including fresh schema and privileges.
- `make web-generate-api-types`: OpenAPI types regenerated; repeated generation
  produced the same file hash.
- `git diff --check`: passed.
