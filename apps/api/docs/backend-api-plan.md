# Backend analytics API plan

Status: implemented foundation and active methodology reference.

## Implemented foundation

- Go service under `apps/api` with environment configuration, structured logs, request IDs, recovery, CORS allowlist, security headers, server timeouts, graceful shutdown, and a bounded PostgreSQL pool.
- Docker Compose service and reproducible pinned Go toolchain.
- OpenAPI contract for operations, geographic hierarchy, overview/time-series
  statistics, outlier audit, and bounded map routes.
- PostGIS-backed geographic hierarchy, package-aware `analytics.sale_facts`,
  versioned outlier evaluations, and refresh audit tables.
- Least-privilege PostgreSQL API role: read access to analytical/source data
  and narrowly scoped writes only for identities, sessions, preferences, and
  authenticated anomaly reviews. Migrations and analytical refreshes remain
  owner-only.
- Unit tests, race checks, vet, CI, Make targets, and VS Code launch/tasks.

The current implementation includes GURS boundary ingestion, the tested
`analytics.sale_facts` classification layer, overview/time-series sale
statistics, robust outlier auditing, and bounded map clusters/points.
Rental analytics and transaction-detail endpoints are implemented.
Distribution, building history, anomaly review, identity/preferences,
repeat-sales and hedonic indices, and transaction-level predictive ranges are
implemented. Regional model indices, richer property attributes, hierarchical
models, and externally reviewed anomaly labels remain future refinements.

This document preserves the agreed direction for the Go backend that will expose Slovenian real-estate sale and rental analytics to the future React frontend.

## Architecture

```text
Python data pipeline
        ↓
Raw PostgreSQL/PostGIS tables
        ↓
Analytical views and materialized views
        ↓
Least-privilege Go REST API
        ↓
React maps and dashboards
```

The Python pipeline remains responsible for downloading and importing source data. The Go application is responsible for validated filters, statistics, spatial queries, and API responses. The frontend must not query raw database tables directly.

## Analytical layer

Create an explicit analytical layer rather than calculating metrics directly from raw joins:

- `analytics.sale_facts`
- `analytics.rental_facts`
- `analytics.geographic_units`
- `analytics.property_geographies`
- `analytics.municipality_year_stats`
- `analytics.property_type_year_stats`

A sale fact should include transaction ID, contract date, dataset year, property type, municipality, cadastral municipality, area, price, price per square metre, PostGIS location, and a price-quality classification.

### Statistical correctness

- A transaction can contain multiple buildings and parcels.
- Never copy the total transaction price onto every associated property row.
- Component prices may be missing.
- The total transaction price can be attributed when the transaction contains exactly one eligible property.
- Missing, zero, or invalid area must not produce a price-per-square-metre value.
- Prefer medians and percentiles over averages for price reporting.
- Every statistic must return its usable sample size.
- Preserve a derivation flag such as `component_price`, `single_property_transaction_price`, or `unavailable`.
- Rental amounts must not be compared until their time basis is understood and normalized.

## Geographic hierarchy

Analytics must support multiple levels rather than treating municipality names as the only geography:

```text
Slovenia
└── statistical region (12)
    └── municipality (212)
        ├── city district / četrtna skupnost where available
        ├── local community / krajevna skupnost where available
        └── settlement / naselje
```

Use the official GURS RPE OGC API as boundary reference data:

- [municipalities](https://ipi.eprostor.gov.si/wfs-si-gurs-rpe/ogc/features/collections/SI.GURS.RPE%3AOBCINE/items?f=application%2Fgeo%2Bjson&limit=1000)
- [statistical regions](https://ipi.eprostor.gov.si/wfs-si-gurs-rpe/ogc/features/collections/SI.GURS.RPE%3ASTATISTICNE_REGIJE/items?f=application%2Fgeo%2Bjson&limit=100)
- `SI.GURS.RPE:CETRTNE_SKUPNOSTI` for city districts
- `SI.GURS.RPE:KRAJEVNE_SKUPNOSTI` for local communities
- `SI.GURS.RPE:NASELJA` for settlements

The inspected current collections contain 12 statistical regions, 212 municipalities, 69 city districts, and 983 local communities. GeoJSON positions are longitude/latitude; store source geometry as EPSG:4326 and a generated/transformed EPSG:3794 geometry for joins with property points.

Names are not stable unique keys. For example, several municipalities contain a district named `Center`. Preserve GURS feature EIDs, codes, composite codes, parent municipality EIDs, names, source timestamps, and geometry. Maribor municipality is code `70`, EID `110200000110278814`; its city districts include `Center` (composite code `70301`) and `Tabor` (`70310`).

Current Maribor boundaries contain 11 city districts. A direct exploratory spatial join found 7,362 sale-building points and 8,056 rental points in Center, and 7,433 sale-building points and 7,118 rental points in Tabor. The boundaries are therefore directly useful for neighborhood analytics.

Import current boundaries into PostGIS and assign geographic IDs to analytical facts during refresh. Do not spatially join millions of points to polygons for each API request. Keep current-boundary analytics as the default so historical time series use comparable geography. Historical `_H` RPE collections may later support an explicit `boundaryVersion=historical` mode.

Not every municipality defines the same sub-municipal unit types. The API must expose which levels are available under each parent instead of promising nationwide city-district coverage.

## Proposed API

All routes are versioned under `/api/v1`.

### Operations and metadata

```http
GET /api/v1/health
GET /api/v1/filters
GET /api/v1/property-types
GET /api/v1/geographies
GET /api/v1/geographies/{id}/children
GET /api/v1/municipalities
GET /api/v1/years
```

### Statistics

```http
GET /api/v1/statistics/overview
GET /api/v1/statistics/timeseries
GET /api/v1/statistics/distribution
GET /api/v1/statistics/municipalities
GET /api/v1/statistics/geographies/{id}
GET /api/v1/statistics/outliers
GET /api/v1/statistics/buildings/{cadastralMunicipalityCode}/{buildingNumber}
```

Filters should include transaction kind, date range, interval (`month`, `quarter`, or `year`), geographic unit, property type, area range, construction-year range, and quality level. Overview responses should expose transaction count, median and average price, median and average price per square metre, median area, and the price-per-square-metre sample size.

Time-series responses must support monthly, quarterly, and yearly intervals. A period is based on `contract_date`, not archive `dataset_year`. Mark incomplete periods such as the current month/year, return sample sizes, and suppress or flag statistically weak cells. Use three- or twelve-month rolling medians when monthly samples are sparse. Distribution responses should initially expose p10, p25, median, p75, and p90.

### Deep-market analysis

The analytical layer should support:

- transaction volume, median price, median price/m², area, and year-over-year/month-over-month changes;
- rolling price and volume trends at national, region, municipality, district, settlement, and building level;
- market liquidity/turnover, days or gaps between repeat sales where identifiable, and new-build share;
- premiums or discounts by construction year, floor, area band, room count, building position, and property type;
- spatial heat maps and comparison of neighboring geographic units;
- repeat-sales indices for the same building part when identifiers are complete;
- building-level median, dispersion, sale/rental activity, and comparison with its district;
- indicative gross rental-yield comparisons by building/district and time band, clearly labeled as modelled rather than matched investment returns;
- data-quality coverage and confidence for every metric.

There are 604,713 sale-building rows with a cadastral municipality and building number, representing about 180,448 identifiable buildings. There are 154,776 transactions containing exactly one building and no parcel; 137,914 of them currently have positive total price and sold area and can support a defensible transaction-price/m² metric. Component-level sale prices are rare (only 869 building rows and 196 parcel rows), so single-property eligibility rules are essential. Rentals have much better component-price coverage: 150,887 rows currently have positive component rent and rented area.

### Outlier analysis

Outliers must be separated into two concepts:

1. **data-quality outliers** — impossible dates, zero/negative price or area, invalid coordinates, implausible construction year, duplicates, or inconsistent transaction totals;
2. **statistical market outliers** — observations whose price or price/m² is economically implausible relative to genuinely comparable properties and would materially distort a representative market statistic. For example, if comparable 50 m² apartments in the same building sell near EUR 100,000 and one is recorded at EUR 800,000, the latter should be flagged for exclusion from the representative average.

Use robust methods rather than a global average: median absolute deviation, interquartile range, and percentile bounds within a comparable cohort. Prefer same-building, same-property-type, similar-area, and nearby-time comparisons when the sample is sufficient; fall back progressively to district and municipality cohorts. Cohorts should also consider construction period, floor, new-build status, and transaction-package classification.

An outlier is excluded from **representative/cleaned** averages and trend calculations only after deterministic, tested eligibility and outlier rules. It remains stored and queryable, is never deleted from source data, and may still appear in raw/audit statistics. The API must distinguish at least:

- `median`: primary robust market measure;
- `cleanedAverage`: mean after eligible statistical outliers are excluded;
- `rawAverage`: optional transparent mean before statistical-outlier exclusion;
- `sampleSize`, `includedSampleSize`, and `excludedOutlierCount`;
- the cohort, bounds, score, and machine-readable exclusion reasons.

Before treating an extreme price as a statistical market outlier, verify that it is not a mixed transaction package, multiple properties, an attributable land/garage component, a new-build or luxury difference, or a source-data error. The API must describe it as statistically atypical rather than alleging fraud or wrongdoing; transaction data alone cannot establish the cause.

Require a minimum cohort size. If a building has too few comparable sales, widen the cohort in controlled steps and report the level used. Thresholds must be evaluated against known examples and covered by unit and database tests before publication.

### Statistical methodology roadmap

Every published result must state its population definition, eligible and excluded sample sizes, time period, geographic level, property/package classification, price derivation, and uncertainty or confidence where applicable. Methods are introduced in stages.

#### Foundation methods

- **Median and weighted quantiles:** primary price, price/m², rent, and area measures; publish p10, p25, p50, p75, and p90 to show the distribution.
- **Interquartile range and median absolute deviation (MAD):** robust dispersion and statistical-outlier detection within comparable cohorts.
- **Cleaned/trimmed mean:** an intuitive average calculated only after deterministic eligibility and statistical-outlier rules; always publish the median and excluded count beside it.
- **Rolling medians:** three-, six-, and twelve-month windows for noisy monthly district or property-type series.
- **Year-over-year and period-over-period change:** calculated from comparable, complete periods with explicit base sample sizes.
- **Bootstrap confidence intervals:** communicate uncertainty around medians, changes, and other estimates without assuming normally distributed property prices.
- **Minimum-sample and disclosure rules:** suppress or clearly mark unstable building/month/cohort results rather than presenting false precision.

#### Comparable-property and price-index methods

- **Hedonic regression:** estimate the contribution of area, property type, construction period, floor, new-build status, location, and time. Large residuals provide a feature-adjusted statistical-outlier signal.
- **Quantile regression:** measure how characteristics affect inexpensive, typical, and premium properties differently instead of modelling only the mean.
- **Repeat-sales index:** calculate price movement from the same building part sold more than once, reducing differences caused by changing property mix.
- **Hedonic time index:** estimate market movement while controlling for property characteristics when repeat-sale coverage is insufficient.
- **Hierarchical/partial-pooling models:** stabilize estimates for small districts or municipalities by sharing information with their parent geography while retaining local differences. Use only when assumptions and uncertainty are clearly exposed.

#### Spatial methods

- **Spatial aggregation:** hexagonal/grid and official-boundary summaries for map visualization without exposing millions of individual points.
- **Local comparable analysis:** distance- or boundary-constrained cohorts around a property, combined with property characteristics and time windows.
- **Spatial autocorrelation:** Moran's I and local indicators to identify statistically significant hot/cold price clusters rather than relying only on visual heat maps.
- **Spatial smoothing:** carefully validated neighborhood estimates for sparse areas, always displaying coverage and uncertainty and never presenting modelled values as observed transactions.

#### Market-structure methods

- **Transaction volume and turnover:** count activity by period, geography, property type, and identifiable building; distinguish market size from price movement.
- **Liquidity proxies:** time between repeat transactions and frequency of sales/rentals in a building or local market.
- **Distribution shift:** compare quantiles and property mix across periods to distinguish genuine price change from a different composition of sold properties.
- **Change-point detection:** flag sustained structural changes in price or volume series, not single noisy months.
- **Sale/rent relationship:** compare robust sale price/m² and normalized rent/m² cohorts to produce an explicitly modelled indicative gross yield range, not a guaranteed investment return.

#### Secondary anomaly models

Isolation Forest, Local Outlier Factor, or other unsupervised anomaly models may be evaluated as secondary review signals after interpretable cohort/MAD and hedonic-residual methods exist. They must never silently remove observations or be the sole reason a transaction is excluded from published statistics. Store model version, features, score, threshold, and explanation metadata for reproducibility.

The complete signal, scoring, validation, and review policy is maintained in
[anomaly-methodology.md](anomaly-methodology.md).

#### Validation requirements

- Back-test methods on historical periods and known hand-reviewed examples.
- Prevent future transactions from leaking into historical estimates.
- Compare results across alternative thresholds, cohorts, and geographic levels.
- Monitor included/excluded percentages and alert on sudden changes after source refreshes.
- Version every analytical rule/model and retain enough metadata to reproduce published results.
- Prefer understandable, stable statistics over a more complex model that cannot be explained to API users.

The current data contains 586 rental contract dates outside the initial valid range used in the audit, including an impossible year `0014`; sales had no dates outside that range. Date validation belongs in the analytical layer and should eventually be moved into ETL validation.

### Map

```http
GET /api/v1/map/clusters?bbox=...&zoom=...
GET /api/v1/map/properties?bbox=...&zoom=...&limit=500
```

Do not send millions of points to the browser. Return clusters or PostGIS grid cells at low zoom levels and capped individual points inside a required bounding box at high zoom levels. GeoJSON is acceptable for the first version; Mapbox Vector Tiles are the likely long-term format.

### Details

```http
GET /api/v1/sales/{transactionId}
GET /api/v1/rentals/{transactionId}
```

A detail response returns the transaction and all associated property rows, coordinates, source provenance, and relevant data-quality information.

## Go implementation direction

```text
apps/api/
├── cmd/api/main.go
├── internal/
│   ├── analytics/
│   ├── config/
│   ├── database/
│   ├── generated/
│   └── http/
├── openapi/
├── queries/
├── Dockerfile
└── go.mod
```

Preferred components:

- standard Go HTTP facilities for the server;
- `pgxpool` for PostgreSQL connections;
- `sqlc` to generate type-safe Go code from reviewed SQL;
- an OpenAPI contract shared with the future frontend;
- structured JSON logs, request validation, timeouts, and graceful shutdown;
- unit tests for statistical rules and PostgreSQL integration tests for queries.

## Performance direction

1. Put interpretation and quality rules in analytical views.
2. Precompute expensive year/municipality/property aggregates with materialized views.
3. Import RPE polygons and preassign statistical-region, municipality, district/community, and settlement IDs to facts.
4. Refresh materialized views and geographic assignments after a successful pipeline import.
5. Use GiST spatial indexes for bounding-box and distance queries.
6. Add B-tree indexes for date, geographic IDs, building identity, property type, and common filter combinations.
7. Precompute monthly/yearly aggregates and robust cohort baselines; never run national polygon joins per request.
8. Add HTTP caching only after measuring database performance.

## First implementation milestone

1. Add a pipeline step and schema for current RPE statistical-region, municipality, and city-district boundaries.
2. Define and test `analytics.sale_facts`, including geographic IDs and quality flags.
3. Scaffold the Go API and its Docker Compose service.
4. Implement health, geography, and filter endpoints.
5. Implement monthly/yearly overview and time-series sale statistics.
6. Implement one bounding-box map endpoint with clustering.
7. Publish and validate the OpenAPI contract.
8. Add unit and PostgreSQL integration tests.
9. Use this vertical slice to support the first React dashboard and map.

## Questions to resolve before implementation

- Map source property-type codes to stable API categories.
- Define eligibility rules for price-per-square-metre calculations.
- Determine the meaning and time basis of every rental price/duration field.
- Define the refresh/version policy for current and historical RPE boundaries.
- Define minimum samples and disclosure thresholds for small regions, buildings, and monthly periods.
- Define privacy/precision rules before exposing individual transaction coordinates publicly.
