# Frontend architecture

## Purpose

`apps/web` is the Slovenian-language **DOMOGLASNIK** dashboard. It is
a Next.js/React/TypeScript client for exploring sale and rental markets; it
does not calculate authoritative statistics locally. Values, publication
status, transaction composition, quality evidence, and geography come from
`real-estate-api`.

## User workflow

### Recent local price rankings and map navigation

The selection inspector shows up to ten highest and ten lowest **total amounts**
from representative transactions, including date, area and amount/m². Rent amounts
are monthly. These are actual GURS agreements, not asking prices or a valuation.
Area/geography selection uses that area. A selected transaction or building uses
its source municipality, not the building-only cohort. Without a known municipality
the property view does not silently substitute a national ranking.

Start with the last three calendar months ending at the earlier of today and the
selected end-year's December 31. If fewer than ten distinct transactions exist,
expand to six, twelve, twenty-four months, then all selected years (at most five
requests, duplicate windows removed). The exact range and expansion are visible.
Keep geography, property group, min/max area and land scope unchanged. Fewer than
ten is valid; the same transaction can occur in both lists. This is **price ranking
within a recent window**, not ten records selected by date and then sorted.

Opening a transaction centers its recorded coordinates at building zoom; opening
a building from the inspector uses its recorded building coordinates. Selecting
a municipality/district from any context control fits that area's actual bounds,
including zooming back out from a property. Cancelled/stale lookups never move the
camera. Missing coordinates/bounds produce an explicit notice, not a guessed point.

### Land browsing versus model population

Sale → Zemljišča exposes separate land categories: all, verified buildable (ETN
1/2), unverified building land (ETN 3), agricultural (ETN 7/8), forest (ETN 9),
mixed-category contracts, and other/unknown uses. The legacy broad (ETN 1/2/3)
scope remains supported for saved URLs, but is not offered as a new category.
The land selector occupies a dedicated full-width toolbar row, stacking inside
the mobile filter panel. Short option labels keep the native control readable;
its linked description retains the full ETN-code and population explanation.
Agricultural and forest filters require all parcels to belong to that category.
A contract with agricultural and forest parcels (or a known category plus an
unknown parcel) belongs to mixed, not to either pure category. Entire package
amounts remain intact and are never allocated proportionally between land uses.
All includes every land-only agreement already stored by the pipeline. There is
no land rental option. The scope persists in shareable URLs and is passed through
market summaries, map features, trends/distribution, geography comparisons,
outliers and recent/full-period price rankings. Area constraints now also flow
through the detailed workspace instead of being lost there.

Publication/quality rules still apply. Mixed-use all-land medians are descriptive,
not a valuation of agricultural or buildable land. Existing valuation models and
national land indices remain based on the strict buildable population; they are
not retrained or broadened by the browsing filter. Anomaly summaries retain their
existing transaction-kind-wide scope.

### Product/repository naming

The frontend title/wordmark and documentation use DOMOGLASNIK. Runtime service,
database, volume, image and Go module names stay stable. GitHub repository renaming
requires an account administrator's Settings → General action. Do not change the
Git remote until that rename has succeeded; then verify SSH access, documentation
links and GHCR workflow package access. Rebranding must not rename Compose volumes.

## Available features

The dashboard provides:

- sale/rent mode switching;
- date, property group, and inclusive minimum/maximum area filters;
- semantic MapLibre navigation from country and statistical regions through
  municipalities/local areas to property/building points;
- street/satellite basemaps, clustering, address search, boundary selection,
  and building-grouped transactions; address results show the active market's
  transaction count within the selected year range and offer an explicit
  one-click filter suggestion when the building has no matching transactions
  but another property group or mode does;
- a market snapshot with explicit median terminology, transaction depth,
  representative share, eligible population, and excluded outliers;
- price/rent trends, year-over-year context, distributions and percentiles;
- a shared trend card with tabs for median amount per m², transaction volume,
  and official Slovenian inflation, selectable by month, quarter, or year;
- geography comparisons, highest/lowest representative transactions,
  anomaly context, indices, and model valuation context;
- detailed sale/rental package composition and building history;
- building-first direct comparables ranked by year and relative area;
- nearby and small-area comparisons matched by floor area and construction
  year, with their actual matching tolerances displayed in the card;
- broad market medians and model estimates kept visually separate from direct
  and physically matched evidence.
- a separate current-market estimate range showing its weighted P25–median–P75
  range, actual adaptive radius, comparable count, physical tolerances,
  location adjustment, confidence and limitations;
- a transaction inspector with progressively disclosed `Pregled`, `Primerljivi
  posli`, `Podrobnosti`, and `Kakovost in model` tabs. Direct observed evidence
  precedes wider benchmarks and the trained model.

Filters define one market population for dashboard and map requests and are
stored in URL query parameters so an analysis can be refreshed or shared.
Selecting another building closes the previous selection. Thin or suppressed
cells are shown honestly rather than replaced with a fabricated median.

Transaction values reported by ETN, calculations made by the application, and
statistical model estimates have separate provenance labels. The persistent
footer provides GURS/ETN/RPE attribution, the active ETN period, a CC BY 4.0
link, non-endorsement wording, valuation limitations, and the privacy/data
minimization statement. These disclosures must remain visible in any public or
commercial deployment.

## Structure

- `app/` contains the Next.js entry point and three ordered style layers:
  `globals.css` holds legacy-compatible structural foundations,
  `redesign.css` owns reusable semantic components and responsive data
  preservation, and `atlas.css` owns the final cartographic composition. Old
  viewport rules must not hide analytical fields and are removed rather than
  counteracted in a later layer.
- `components/dashboard/market-dashboard.tsx` owns URL/filter/selection state
  and request cancellation.
- `filter-bar.tsx` owns primary/secondary market controls and the accessible
  mobile filter sheet, including focus restoration and Escape handling.
- `slovenia-map.tsx` owns MapLibre sources, semantic zoom, clusters,
  boundaries, search, and selection.
- `market-snapshot.tsx` renders the selected population's primary evidence.
- `context-panel.tsx` loads selection data and delegates transaction rendering
  to `components/dashboard/inspector/`; reusable evidence primitives live in
  `components/ui/`.
- trend, distribution, and analytics-workspace components render focused
  analytical views. Detailed analytics fetch only the active tab and cache
  previously loaded tab/filter combinations.
- `lib/api.ts` is the typed HTTP boundary; `lib/types.ts` contains UI-facing
  domain types and generated OpenAPI types live under `lib/generated`.
- colocated Vitest/Testing Library tests characterize components; pure display
  and map-popup helpers have focused unit tests.

## API and failure behavior

`NEXT_PUBLIC_API_URL` selects the backend (default local value
`http://localhost:8080`). Requests use abort signals to prevent stale filter
responses from overwriting a newer selection. API errors and unavailable
statistics produce visible states. The frontend never replaces unavailable
live statistics with demo values.

Refreshes are local to the affected panel. Existing successful data remains
readable while a newer request is running. Suppressed periods create gaps in
price lines, and suppressed distributions do not render synthetic histograms.

The checked-in TypeScript transport schema is generated from
`apps/real-estate-api/openapi/openapi.yaml`. CI fails on contract drift.

## Map behavior

The frontend requests bounded GeoJSON rather than loading Slovenia's complete
transaction history into browser memory. At broad zoom it requests named
administrative summaries; at intermediate zoom it requests server-side grid
clusters; at close zoom it requests capped individual points. Multiple
transactions at the same cadastral building are presented together.

Map points represent GURS property centroids. They may differ from postal
entrances and are omitted if coordinates are unavailable.

## Development and verification

```bash
make web-up
make web-lint
make web-typecheck
make web-test
make web-build
make web-generate-api-types
make web-contract-drift
```

Open <http://localhost:3000>. A local Node installation is optional because
the Make targets execute in Docker. UI changes should include an interaction
test, API-boundary changes an API helper/contract test, and map behavior a
focused map/helper test where practical.

To rebuild only the frontend without starting the scheduled pipeline workflow:

```bash
docker compose up -d --build --force-recreate --no-deps web
```

`docker compose up --wait web` includes Compose dependencies and can recreate
`pipeline-worker`; that worker performs a refresh immediately when it starts.
