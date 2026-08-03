# Frontend architecture

## Purpose

`apps/web` is the Slovenian-language **Analitika nepremičnin** dashboard. It is
a Next.js/React/TypeScript client for exploring sale and rental markets; it
does not calculate authoritative statistics locally. Values, publication
status, transaction composition, quality evidence, and geography come from
`real-estate-api`.

## User workflow

The dashboard provides:

- sale/rent mode switching;
- date, property group, and inclusive minimum/maximum area filters;
- semantic MapLibre navigation from country and statistical regions through
  municipalities/local areas to property/building points;
- street/satellite basemaps, clustering, address search, boundary selection,
  and building-grouped transactions;
- overview metrics with cleaned/raw sample sizes and publication warnings;
- price/rent trends, year-over-year context, distributions and percentiles;
- a shared trend card with tabs for median amount per m², transaction volume,
  and official Slovenian inflation, selectable by month, quarter, or year;
- geography comparisons, highest/lowest representative transactions,
  anomaly context, indices, and model valuation context;
- detailed sale/rental package composition and building history.
- building-first direct comparables ranked by year and relative area;
- nearby and small-area comparisons matched by floor area and construction
  year, with their actual matching tolerances displayed in the card;
- broad market medians and model estimates kept visually separate from direct
  and physically matched evidence.
- a separate current-market estimate card showing its weighted p25–median–p75
  range, actual adaptive radius, comparable count, physical tolerances,
  location adjustment, confidence and limitations.

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

- `app/` contains the Next.js entry point, layout, and global design tokens.
- `components/dashboard/market-dashboard.tsx` owns URL/filter/selection state
  and request cancellation.
- `filter-bar.tsx` owns accessible market controls.
- `slovenia-map.tsx` owns MapLibre sources, semantic zoom, clusters,
  boundaries, search, and selection.
- summary, trend, distribution, context, and analytics-workspace components
  render focused analytical views.
- `lib/api.ts` is the typed HTTP boundary; `lib/types.ts` contains UI-facing
  domain types and generated OpenAPI types live under `lib/generated`.
- colocated Vitest/Testing Library tests characterize components; pure display
  and map-popup helpers have focused unit tests.

## API and failure behavior

`NEXT_PUBLIC_API_URL` selects the backend (default local value
`http://localhost:8080`). Requests use abort signals to prevent stale filter
responses from overwriting a newer selection. API errors and unavailable
statistics produce visible states. Representative demo data is used only for
the explicit standalone preview fallback and is labelled as such.

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
