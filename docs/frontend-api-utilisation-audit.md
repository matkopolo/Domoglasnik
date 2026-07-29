# Phase 2 frontend/API utilisation audit

Date: 2026-07-29

## Scope

This audit was completed against the current branch before Phase 2 production
changes. The authoritative route inventory is
`apps/api/internal/httpapi/routes.go`; before Phase 2 it contained 37
method/path operations. Phase 2 adds the two transaction-context operations
listed below (39 operations across 38 paths after implementation).
The frontend currently calls 17 operations, but several responses are only
partially typed or rendered.

Phase 1 definitions remain authoritative. In particular,
`propertyGroup=land` defaults to strict ETN codes 1–2, unpublishable cells
retain counts and warnings, and frontend code must not recalculate analytical
populations from visible map markers.

Status terms:

- **used** — directly called by the current frontend;
- **partial** — called, but useful response fields or interaction are omitted;
- **missing** — valuable frontend integration does not exist;
- **backend-only** — deliberately not public dashboard UI;
- **analyst-only** — dedicated authenticated review workflow;
- **operational** — service/deployment health.

## Complete endpoint matrix

| # | Method and path | Handler / store | Before | Payload use | Audience | Phase 2 decision | Required verification |
|---:|---|---|---|---|---|---|---|
| 1 | `GET /health/live` | `live`; no store | operational | n/a | operational | Keep backend-only | route/OpenAPI |
| 2 | `GET /health/ready` | `ready`; database pinger | operational | n/a | operational | Keep backend-only; deployment health | readiness tests |
| 3 | `GET /api/v1/health` | `ready`; database pinger | operational | n/a | operational | Keep backend-only compatibility alias | route/OpenAPI |
| 4 | `GET /api/v1/meta` | `meta` | missing | none | public dashboard | Integrate freshness/capability metadata | loading/error test |
| 5 | `GET /api/v1/geographies` | `listGeographies`; geography store | used | partial: region selector only | public dashboard | Retain initial country/region discovery | hierarchy tests |
| 6 | `GET /api/v1/geographies/{id}` | `getGeography`; geography store | missing | none | public dashboard | Integrate selected geography identity and parent navigation | 404 and selection tests |
| 7 | `GET /api/v1/geographies/{id}/boundary` | `geographyBoundary`; geography store | used | full geometry, low detail only | public dashboard | Retain map boundary rendering and cancellation | abort/cache tests |
| 8 | `GET /api/v1/geographies/{id}/children` | `geographyChildren`; geography store | missing | none | public dashboard | Integrate drilldown and child list | region→municipality→local-area test |
| 9 | `GET /api/v1/statistics/overview` | `statisticsOverview`; statistics store | used | partial: only core medians/counts | public dashboard | Integrate full sample, cleaned/raw, publication metadata | exact filter and weak-cell tests |
| 10 | `GET /api/v1/statistics/timeseries` | `statisticsTimeSeries`; statistics store | used | partial: ignores incomplete/publication/raw fields | public dashboard | Integrate warnings and backend points | stale-response test |
| 11 | `GET /api/v1/statistics/outliers` | `statisticsOutliers`; statistics store | missing | none | public analytical detail | Keep endpoint available; transaction detail evidence is sufficient for primary panel, dedicated public list deferred | endpoint classified; no public queue confusion |
| 12 | `GET /api/v1/rentals/statistics/overview` | `rentalOverview`; rental store | used | partial | public dashboard | Integrate full sample/publication metadata | rental panel tests |
| 13 | `GET /api/v1/rentals/statistics/timeseries` | `rentalTimeSeries`; rental store | used | partial: existing type uses wrong YoY field name | public dashboard | Correct DTO and warnings | contract/type tests |
| 14 | `GET /api/v1/rentals/statistics/outliers` | `rentalOutliers`; rental store | missing | none | public analytical detail | Same deliberate deferral as sale outlier list | classification documented |
| 15 | `GET /api/v1/rentals/map/clusters` | `rentalMapClusters`; rental store | used | initial cluster payload only | public map | Retain | map request tests |
| 16 | `GET /api/v1/rentals/map/geographies` | `rentalMapGeographies`; map store | used | map label/count/median | public map | Retain and use selection context | semantic zoom/selection tests |
| 17 | `GET /api/v1/rentals/map/properties` | `rentalMapProperties`; rental store | used | markers only; no detail action | public map | Integrate rental transaction selection | rental detail test |
| 18 | `GET /api/v1/sales/transactions/{id}` | `saleTransaction`; transaction store | used | partial: first component and a few fields | public transaction panel | Integrate composition, quality, outlier, hierarchy, derivation, land | whitelist/display tests |
| 19 | `GET /api/v1/rentals/transactions/{id}` | `rentalTransaction`; transaction store | missing | none | public transaction panel | Integrate now | rental detail/empty tests |
| 20 | `GET /api/v1/statistics/distribution` | `saleDistribution`; distribution store | used | partial: assumes non-null percentiles | public dashboard | Integrate publication/empty states | empty/weak tests |
| 21 | `GET /api/v1/rentals/statistics/distribution` | `rentalDistribution`; distribution store | used | partial | public dashboard | Integrate publication/empty states | rental distribution tests |
| 22 | `GET /api/v1/buildings/{cadastralCode}/{buildingNumber}/statistics` | `buildingStatistics`; building store | used | partial: sales only, unfiltered | public building panel | Extend with mode/property/date filters and render histories/rentals | filter/integration tests |
| 23 | `GET /api/v1/buildings/search` | `searchBuildings`; building store | used | most search fields | public map | Retain; selection must become shared context | abort/keyboard tests |
| 24 | `GET /api/v1/anomalies` | `anomalies`; anomaly store | analyst-only | none | analyst/admin | Dedicated authenticated screen deferred; never put raw review queue in public dashboard | classification only |
| 25 | `GET /api/v1/anomalies/summary` | `anomalySummary`; anomaly store | analyst-only | none | analyst/admin | Same dedicated-screen deferral | classification only |
| 26 | `PATCH /api/v1/anomalies/{kind}/{id}/review` | `reviewAnomaly`; anomaly store | analyst-only | none | analyst/admin | Defer with analyst screen/auth | authorization remains tested |
| 27 | `POST /api/v1/auth/register` | `register`; identity store | backend-only | none | authenticated users | Defer until a coherent saved-view product and Phase 3 CORS verification | existing auth tests |
| 28 | `POST /api/v1/auth/login` | `login`; identity store | backend-only | none | authenticated users | Same deferral | existing auth tests |
| 29 | `POST /api/v1/auth/logout` | `logout`; identity store | backend-only | none | authenticated users | Same deferral | existing auth tests |
| 30 | `GET /api/v1/users/me` | `me`; identity store | backend-only | none | authenticated users | Same deferral | existing auth tests |
| 31 | `GET /api/v1/users/me/preferences` | `getPreferences`; identity store | backend-only | none | authenticated users | Defer saved filters/favourites | existing auth tests |
| 32 | `PUT /api/v1/users/me/preferences` | `savePreferences`; identity store | backend-only | none | authenticated users | Defer saved filters/favourites | existing auth tests |
| 33 | `GET /api/v1/indices` | `marketIndices`; model store | missing | none | public analytical panel | Integrate national quality-adjusted index with explicit scope/method | weak-period and label tests |
| 34 | `GET /api/v1/valuations/{kind}/{id}` | `valuationPrediction`; model store | missing | none | public transaction panel | Integrate interval, reliability, model warnings; neutral 404 state | reliability/missing tests |
| 35 | `GET /api/v1/map/clusters` | `mapClusters`; map store | used | initial cluster payload only | public map | Retain | map request tests |
| 36 | `GET /api/v1/map/geographies` | `mapGeographies`; map store | used | labels/count/median; click does not select marker geography | public map | Make marker click select exact geography | hierarchy tests |
| 37 | `GET /api/v1/map/properties` | `mapProperties`; map store | used | markers grouped by building in browser | public map | Retain; marker opens shared building/transaction context | stale/detail tests |
| 38 | `GET /api/v1/sales/transactions/{id}/context` | `saleTransactionContext`; transaction store | new | same-year representative geographic and nearby benchmarks, quality, valuation | public detail | Implement and consume in persistent context panel | unit/integration/handler/UI |
| 39 | `GET /api/v1/rentals/transactions/{id}/context` | `rentalTransactionContext`; transaction store | new | same-year representative geographic and nearby benchmarks, quality, valuation | public detail | Implement and consume in persistent context panel | unit/integration/handler/UI |

## Confirmed partial-payload gaps

### Dashboard statistics

The transport types omit or fail to render raw versus cleaned averages, metric
sample sizes, median area, incomplete state, publication state, and
suppression reason. `SummaryPanel` also substitutes zero when a median amount
is absent, which is not an acceptable missing-value policy.

### Sale transaction

The frontend selects only `buildingParts[0]`, displays missing values as
`Ni podatka`, and discards dataset year, derivation, quality, outlier,
geography, most component fields, and all meaningful land fields. Multiple
parts are summarized rather than presented as composition.

### Rental transaction

The API is complete enough for an initial public panel, but there is no
frontend call or DTO.

### Building

The API returns sale/rental summaries and histories, but the frontend type and
card retain only sale summary and transactions. The backend does not accept
active property-group or date filters, so current building medians can mix
apartments, garages, commercial units, and years.

### Geography

Map geography summaries are used for labels, but named parent/child navigation
and exact selected geography identity are absent. Clicking a semantic marker
zooms without selecting it; only boundary-fill clicks alter statistics.

### Transport failure

`loadDashboard` catches every non-abort failure and silently returns fixed demo
statistics. The UI can therefore label invented values as a live market view.
Phase 2 requires an explicit error/stale state while preserving the last
successfully loaded live context.

## Phase 2 context and API contract

One `MarketContext` will be shared by the dashboard, map, panel, and URL:

```ts
type MarketContext =
  | { kind: "country"; geographyId?: number }
  | { kind: "geography"; geographyId: number; level: GeographyLevel }
  | { kind: "building"; cadastralCode: number; buildingNumber: string }
  | { kind: "transaction"; transactionKind: "sale" | "rent"; transactionId: number };
```

Map zoom selects the semantic layer; it does not overwrite an explicit
context. Context requests use an `AbortController` plus request-generation
guard so an older response cannot replace a newer selection.

Required backend extensions:

1. `GET /api/v1/{sales|rentals}/transactions/{id}/context` for same-year,
   same-property-group, representative-only nearby and geography benchmarks;
2. `propertyGroup`, `from`, and `to` on building statistics;
3. rental transactions in the building response;
4. codebook descriptions on whitelisted transaction component fields.

No browser aggregation of currently loaded markers will be used as a market
benchmark.

## Implemented disposition

The endpoint matrix remains the before/decision audit. The resulting Phase 2
frontend uses 24 operations, including the two new context operations:

- geography identity, children, boundaries, and batched geography summaries
  drive region, municipality, and supported local-area contexts;
- both sale and rental transaction detail endpoints drive one shared panel;
- filtered building statistics and building search drive the building context;
- the national hedonic index is shown for country context;
- valuation fields are delivered by the transaction-context composite. The
  standalone valuation route remains available, but is deliberately not called
  a second time because that would duplicate work and could combine different
  model vintages in one panel;
- public sale/rental outlier lists remain deliberately unused; the selected
  transaction's quality and outlier evidence is shown instead;
- the authenticated anomaly queue, identity, and preferences operations remain
  analyst-only or backend-only as classified;
- `/api/v1/meta` remains intentionally unused. Its current response is a static
  capability list and contains no source-refresh timestamp. The UI therefore
  shows the actual ETN/RPE source and active coverage period without presenting
  a false refresh time. Adding import freshness requires a future API contract
  backed by `source_files`, not a browser guess.

No endpoint is unclassified. Operations described as missing in the **Before**
column are either integrated above or retain the explicit audience/deferral in
the matrix.
