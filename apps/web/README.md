# Analitika nepremičnin

Next.js frontend for the Slovenian real-estate analytics platform. It consumes
the Go API from `NEXT_PUBLIC_API_URL` and falls back to clearly labelled
representative demo data when that API is unavailable.

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
- viewport-aware API requests switch to individual properties at street zoom.

Trend charts include formatted price-axis ticks and mouse, focus, and keyboard
accessible point tooltips.

The current public tile sources are intended for local development and require
no API key. Their URLs and attribution live in `lib/map-styles.ts`, isolated
from the map component so a licensed production provider can replace them
later. Live clusters, property points, boundaries, and statistics come from
the Go API.

## Future: AI comparator

The planned comparator will accept natural-language opportunities, such as
buildable land with an existing building permit. Before analysis, the UI will
show the interpreted location and attributes for confirmation. Results will
show the estimated premium or discount against suitable nearby comparables,
price range, sample size, confidence, assumptions, and comparable transactions.
Building-permit status must be verified through an authoritative future source
or clearly labelled user evidence; it will not be inferred from ETN prices.
