# Phase 2 frontend/API utilisation and contextual analytics report

Date: 2026-07-29

## Outcome

Phase 2 is complete. The dashboard now uses one persistent, URL-addressable
market context across the filters, map, search, summary, charts, and detail
panel. It builds on the Phase 1 representative/quality definitions and does
not derive market benchmarks from visible markers.

The implemented hierarchy is:

1. Slovenia;
2. statistical region;
3. municipality;
4. city district, local community, or settlement when the RPE hierarchy
   supplies that level;
5. cadastral building;
6. sale or rental transaction.

No Phase 3 general refactoring was undertaken.

## API and analytical implementation

### Transaction context

Two documented public operations were added:

- `GET /api/v1/sales/transactions/{id}/context`;
- `GET /api/v1/rentals/transactions/{id}/context`.

For a selected subject, the repository returns same-contract-year,
same-`property_group`, representative-only cohorts. The subject transaction is
excluded. Sale cohorts read the strict-land population established in Phase 1;
rental cohorts read `analytics.rental_facts`.

Nearby comparisons use PostGIS distance from the subject point and widen
deterministically through 500 m, 1 km, 2.5 km, and 5 km until at least five
peers exist. Geography comparisons use the assigned city district, local
community, settlement, municipality, statistical region, and country where
available. Every returned benchmark includes:

- cohort median amount per m²;
- backend-computed percentage difference;
- percentile;
- contract year;
- sample size;
- reliability tier;
- publication flag;
- radius for a nearby cohort.

Cohorts below five observations are omitted. Samples 5–9 are `very_low`,
10–29 are `low`, and 30 or more are `moderate` and publishable under the
existing descriptive threshold. An active calendar year is marked incomplete.

The response also contains the Phase 1 quality decision, outlier status, and a
public valuation subset. Internal model metrics and reviewer data are not
duplicated into the public context payload.

### Building context

The existing building-statistics operation now accepts the same
`propertyGroup`, `from`, and `to` filters as the dashboard. Summary, history,
and transaction queries all use those filters. Sale data uses strict-land
facts; rental transactions and rental history are now returned alongside
sales.

### Human-readable component data

Transaction component queries resolve descriptions from the imported
codebooks for:

- sale building-part type;
- sale land type;
- rental space type.

The public sale DTO no longer exposes the apartment/business unit number.
Free-text notes and anomaly reviewer identity remain outside the public
contract.

### Existing endpoint utilisation

The panel now consumes geography identity/children, batched map geography
summaries, both transaction detail resources, filtered building statistics,
building search, and the current national hedonic index. The full disposition
of all 39 operations is in `frontend-api-utilisation-audit.md`.

The standalone valuation endpoint is intentionally not fetched after the
context composite has returned the same valuation vintage. Public outlier
lists are intentionally not treated as an error list; selected transaction
quality evidence is shown in context. Authenticated review and preference
operations retain their analyst/user audiences.

`/api/v1/meta` cannot supply source freshness: it contains only static
capabilities. The frontend labels GURS ETN/RPE as the source and displays the
active coverage period. A real import timestamp is deferred until the API
derives one from source-file imports.

## Frontend implementation

`MarketContext` is a discriminated union for country, geography, building, and
transaction selections. Changing a semantic map marker, boundary, search
result, child geography, building transaction, or breadcrumb updates this
single state. Selecting a new context replaces the old panel and closes the
previous map popup.

The URL persists:

- sale/rent mode;
- property group;
- from/to years;
- context kind and identity.

This makes a filtered context reloadable and shareable locally.

The persistent React context panel provides:

- national summary and current hedonic index;
- geography identity, child navigation, and batched peer summaries;
- date/property-filtered sale or rental building history and transactions;
- sale and rental transaction details;
- geographic breadcrumbs;
- complete sale-part, parcel, and rental-space composition;
- contextual market comparisons with sample/reliability;
- quality/outlier evidence;
- valuation interval and warning.

Map popups remain compact and never render floor or position. Detailed
`positionInBuilding` is shown only when meaningful; the generic value
`nadstropje` is omitted.

The previous silent demo-data fallback was removed. Failed requests preserve
the last successful live response and display an explicit error. Dashboard,
map, hierarchy, search, and detail requests use `AbortController`; dashboard,
map, and context loads also use generation guards so stale responses cannot
overwrite a newer selection.

## Property display policy

One tested display-value helper applies the public policy:

- null, undefined, blank, whitespace, and source sentinels are omitted;
- numeric zero remains meaningful;
- absent price/rent is never formatted as zero;
- generic position labels are suppressed;
- codebook descriptions are preferred over raw numeric codes;
- composition is shown per building part, parcel, or rental space;
- optional fields do not create ordinary `Ni podatka` rows.

The source, semantics, availability measurements, public whitelist, and
deferrals are recorded in `frontend-property-field-audit.md`.

## Contract and test coverage

Backend coverage includes:

- percentage-difference and reliability boundary unit tests;
- same-year representative context integration;
- subject exclusion through repository predicates;
- minimum sample and period assertions;
- building filter integration;
- invalid filter handler tests;
- sale/rental context route tests;
- runtime/OpenAPI route parity.

Frontend coverage includes:

- sale/rental map request selection;
- grouping multiple transactions at one building;
- explicit API failure without demo fallback;
- 11 property display-policy cases;
- rental transaction/context rendering;
- URL filter persistence;
- stale dashboard response rejection;
- chart rendering and production TypeScript compilation.

## Performance evidence

Plans were inspected against the populated local ETN database:

- nearby 2.5 km cohort: GiST `sale_facts_location_gist_idx`, 1.95 ms;
- filtered building lookup: `sale_facts_building_idx`, 0.03 ms;
- national apartment/year cohort before the final date-range predicate
  improvement: 156.6 ms including cold reads and JIT.

The context query now uses an indexable date range rather than
`extract(year ...)`. A live composite context request returned HTTP 200 in
0.20 s. A filtered building request returned HTTP 200 in 0.47 s while the
service was concurrently serving map traffic. No new database index was
required.

## Completion gate

- [x] frontend/API utilisation audit exists and classifies all operations.
- [x] property-field audit exists.
- [x] this implementation report exists.
- [x] region, municipality, supported local-area, building, and transaction
  contexts are implemented.
- [x] contextual percentage comparisons are backend-computed and tested.
- [x] missing optional fields use one tested omission policy.
- [x] map/detail requests cancel and reject stale results.
- [x] OpenAPI/runtime parity, Go vet/race tests, frontend lint/tests/type check,
  and production build pass.
- [x] all migrations apply in filename order to a clean temporary database
  (38 public/analytics tables verified); the temporary database was removed.
- [x] contextual query plans were inspected.
- [x] Phase 2 audit, backend, frontend, and report changes use separate focused
  commits.

## Deferrals and remaining risks

1. Source import freshness requires a new database-backed metadata field.
   Displaying the active coverage end year is accurate; inventing a refresh
   timestamp is not.
2. RPE hierarchy coverage varies by municipality. The panel exposes only
   actual children and explicitly reports when none exist.
3. Nearby cohorts with 5–9 observations are useful for orientation but are
   labelled very low reliability and not publishable.
4. Valuation reliability can be low or very low. The interval and “not an
   official valuation” warning remain visible.
5. The existing dependency audit reports nine high-severity npm findings.
   Dependency remediation is Phase 3 code-quality/security work and was not
   mixed into this phase.
6. Keyboard focus movement from a map marker into the context panel can be
   improved during accessibility-focused Phase 3 work; all panel actions are
   native buttons and keyboard-operable now.

## Follow-up analytical workspace

The dashboard now includes a compact four-tab analytical workspace for market
depth, quality-adjusted indices/model diagnostics, geography rankings, and
outlier evidence. Counts explicitly distinguish all recorded transactions
from the representative population used for medians.

A dedicated transaction-extremes API calculates the three highest and three
lowest representative transactions for the active geography, date range, and
property group. Extremes are ranked by total sale price (or monthly rent), not
by visible map markers. Each row also shows amount per square metre and area
and opens the full transaction detail.

The main desktop map has a fixed 610 px card height. The right summary/context
column scrolls independently when its content is longer, preventing the map
from changing size. The contextual and analytical sections use reduced
spacing and typography; mobile layout returns to natural document flow.
