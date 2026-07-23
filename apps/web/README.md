# Web dashboard

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

The map is a lightweight, accessible SVG analytical surface. It does not
depend on commercial map tokens. Live clusters and statistics come from the
Go API; the local outline provides a stable low-bandwidth and offline
substrate.
