# Analitika nepremičnin

Next.js frontend for the Slovenian real-estate analytics platform. It consumes
the Go API from `NEXT_PUBLIC_API_URL`. If that API is unavailable, the
dashboard shows an explicit error state; it does not substitute generated or
demo market values.

The dashboard polls `/api/v1/data-status` every 30 seconds. When newer GURS
data is being processed, it continues displaying the last complete analytical
release and clearly shows both cutoffs and the active pipeline stage. Failed
updates do not replace otherwise usable published market data.

## Local development

```bash
cd apps/web
npm install
npm run dev
```

Open <http://localhost:3000>. The default API is <http://localhost:8080>;
override it with:

```bash
NEXT_PUBLIC_API_URL=https://api.example.si npm run dev
```

## Quality checks

```bash
npm run lint
npm test
npm run build
```

The dashboard uses MapLibre GL JS for a real interactive map:

- street and satellite development basemaps;
- mouse, trackpad, touch, and keyboard navigation;
- exact GURS statistical-region boundaries;
- semantic zoom summaries: Slovenia, statistical regions, municipalities,
  city districts (with local-community fallback), then individual properties;
- named geography markers show the robust median amount per m² and transaction
  count;
- transaction popups show area, total sale price, date, address, construction
  year, and additional building-part area when those values exist in GURS;
- selectable from/to years update the map, overview, distribution, and trend
  chart together;
- inclusive minimum/maximum area filters update the same market population and
  are preserved in the shareable URL;
- address search zooms directly to a building and exposes its individually
  selectable sale transactions;
- street-level sales sharing a cadastral building are consolidated into one
  marker whose popup lists the complete building sale history;
- transaction context ranks individual same-building transactions first,
  then uses strict ±15% area and ±8 construction-year similarity for nearby evidence;
- source ETN values, deterministic calculations, and trained model evidence
  use distinct provenance treatments;
- the transaction inspector separates overview, direct comparables, source
  details, and quality/model diagnostics without discarding fields on mobile;
- the current-market estimate is visualized as a weighted P25–median–P75
  range and is explicitly distinguished from a formal confidence interval;
- viewport-aware API requests switch to individual properties at street zoom.

Trend charts include formatted axes, bars for volume, explicit incomplete and
suppressed periods, and mouse/focus/click-pinned keyboard-accessible tooltips.
Detailed analytics fetches the active tab on demand and caches already loaded
tab/filter combinations.

The current public tile sources are intended for local development and require
no API key. Their URLs and attribution live in `lib/map-styles.ts`, isolated
from the map component so a licensed production provider can replace them
later. Live clusters, property points, boundaries, and statistics come from
the Go API.
