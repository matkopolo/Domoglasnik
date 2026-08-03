"use client";

import { Menu, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { loadBuildingStatistics, loadDashboard, loadTransaction } from "@/lib/api";
import type {
  AreaRange,
  DashboardData,
  GeographyLevel,
  MarketContext,
  MarketMode,
  PropertyGroup,
  TransactionGeography,
  YearRange,
} from "@/lib/types";
import { Brand } from "@/components/ui/brand";
import { ContextPanel } from "./context-panel";
import { AnalyticsWorkspace } from "./analytics-workspace";
import { DistributionChart } from "./distribution-chart";
import { FilterBar } from "./filter-bar";
import { SloveniaMap } from "./slovenia-map";
import { SummaryPanel } from "./summary-panel";
import { TrendChart } from "./trend-chart";

const EMPTY_DATA: DashboardData = {
  overview: {
    transactionCount: 0,
    eligibleCount: 0,
    includedSampleSize: 0,
    excludedOutlierCount: 0,
  },
  timeSeries: [],
  distribution: {
    metric: "pricePerSqm",
    sampleSize: 0,
    isPublishable: false,
    belowBoundCount: 0,
    aboveBoundCount: 0,
    buckets: [],
  },
  clusters: [],
  geographies: [],
  source: "live",
};

type GeographyContext = Extract<MarketContext, { kind: "geography" }>;

const localGeographyPreference: GeographyLevel[] = [
  "city_district",
  "local_community",
  "municipality",
  "settlement",
  "statistical_region",
];

function preferredTransactionGeography(
  geographies: TransactionGeography[],
): GeographyContext | undefined {
  for (const level of localGeographyPreference) {
    const geography = geographies.find((item) => item.level === level);
    if (geography) return { kind: "geography", ...geography };
  }
  return undefined;
}

function initialState(): {
  mode: MarketMode;
  propertyGroup: PropertyGroup;
  years: YearRange;
  context: MarketContext;
  geographyContext?: GeographyContext;
  area: AreaRange;
} {
  if (typeof window === "undefined") {
    return {
      mode: "sale" as MarketMode,
      propertyGroup: "apartment" as PropertyGroup,
      years: { from: 2018, to: 2025 },
      context: { kind: "country" } as MarketContext,
      geographyContext: undefined,
      area: {},
    };
  }
  const query = new URLSearchParams(window.location.search);
  const mode: MarketMode = query.get("mode") === "rent" ? "rent" : "sale";
  const allowed = ["apartment", "house", "land", "commercial", "garage", "room"];
  const group = query.get("propertyGroup");
  const propertyGroup = allowed.includes(group ?? "") ? group as PropertyGroup : "apartment";
  const from = Number(query.get("from")) || 2018;
  const to = Number(query.get("to")) || 2025;
  const min = Number(query.get("minArea")) || undefined;
  const max = Number(query.get("maxArea")) || undefined;
  const contextKind = query.get("context");
  const geographyContext = Number(query.get("geographyId"))
    ? {
        kind: "geography" as const,
        id: Number(query.get("geographyId")),
        level: (query.get("geographyLevel") ?? "statistical_region") as GeographyLevel,
        name: query.get("geographyName") ?? "Izbrano območje",
      }
    : undefined;
  let context: MarketContext = { kind: "country" };
  if (contextKind === "geography" && geographyContext) {
    context = geographyContext;
  } else if (contextKind === "building" && Number(query.get("cadastralCode")) && query.get("buildingNumber")) {
    context = {
      kind: "building",
      cadastralCode: Number(query.get("cadastralCode")),
      buildingNumber: query.get("buildingNumber")!,
    };
  } else if (contextKind === "transaction" && Number(query.get("transactionId"))) {
    context = {
      kind: "transaction",
      mode: query.get("transactionMode") === "rent" ? "rent" : "sale",
      id: Number(query.get("transactionId")),
    };
  }
  return { mode, propertyGroup, years: { from, to }, context, geographyContext, area: { min, max } };
}

export function MarketDashboard() {
  const initial = useMemo(() => initialState(), []);
  const [mode, setMode] = useState(initial.mode);
  const [propertyGroup, setPropertyGroup] = useState(initial.propertyGroup);
  const [years, setYears] = useState<YearRange>(initial.years);
  const [area, setArea] = useState<AreaRange>(initial.area);
  const [context, setContext] = useState<MarketContext>(initial.context);
  const [geographyContext, setGeographyContext] = useState<GeographyContext | undefined>(
    initial.geographyContext,
  );
  const [data, setData] = useState<DashboardData>(EMPTY_DATA);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const generation = useRef(0);
  const contextGeographyGeneration = useRef(0);
  const geographyId = geographyContext?.id;

  useEffect(() => {
    const controller = new AbortController();
    const requestGeneration = ++generation.current;
    queueMicrotask(() => {
      if (!controller.signal.aborted) {
        setLoading(true);
        setError(undefined);
      }
    });
    loadDashboard(mode, propertyGroup, geographyId, years, controller.signal, area)
      .then((result) => {
        if (requestGeneration === generation.current) setData(result);
      })
      .catch((requestError: unknown) => {
        if (!controller.signal.aborted && requestGeneration === generation.current) {
          setError(requestError instanceof Error
            ? requestError.message
            : "Podatkov ni bilo mogoče naložiti.");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted && requestGeneration === generation.current) setLoading(false);
      });
    return () => controller.abort();
  }, [mode, propertyGroup, geographyId, years, area]);

  useEffect(() => {
    const query = new URLSearchParams();
    query.set("mode", mode);
    query.set("propertyGroup", propertyGroup);
    query.set("from", String(years.from));
    query.set("to", String(years.to));
    if (area.min != null) query.set("minArea", String(area.min));
    if (area.max != null) query.set("maxArea", String(area.max));
    query.set("context", context.kind);
    if (geographyContext) {
      query.set("geographyId", String(geographyContext.id));
      query.set("geographyLevel", geographyContext.level);
      query.set("geographyName", geographyContext.name);
    }
    if (context.kind === "building") {
      query.set("cadastralCode", String(context.cadastralCode));
      query.set("buildingNumber", context.buildingNumber);
    } else if (context.kind === "transaction") {
      query.set("transactionMode", context.mode);
      query.set("transactionId", String(context.id));
    }
    window.history.replaceState(null, "", `?${query}`);
  }, [area, context, geographyContext, mode, propertyGroup, years]);

  const selectedName = geographyContext?.name ?? "Slovenija";
  const analysisContext: MarketContext = geographyContext ?? { kind: "country" };
  const syncLocalGeography = (next: Extract<MarketContext, { kind: "building" | "transaction" }>) => {
    const requestGeneration = ++contextGeographyGeneration.current;
    const controller = new AbortController();
    const detail = (async () => {
      if (next.kind === "transaction") {
        return loadTransaction(next.mode, next.id, controller.signal);
      }
      const building = await loadBuildingStatistics(
          next.cadastralCode,
          next.buildingNumber,
          propertyGroup,
          years,
          controller.signal,
      );
      const transaction = mode === "sale"
        ? building.saleTransactions[0]
        : building.rentalTransactions[0];
      return transaction
        ? loadTransaction(mode, transaction.transactionId, controller.signal)
        : undefined;
    })();
    detail.then((transaction) => {
      if (!transaction || requestGeneration !== contextGeographyGeneration.current) return;
      const geography = preferredTransactionGeography(transaction.geographies);
      if (geography) setGeographyContext(geography);
    }).catch(() => undefined);
  };
  const handleContext = (next: MarketContext) => {
    if (next.kind === "geography") {
      contextGeographyGeneration.current += 1;
      setGeographyContext(next);
    }
    if (next.kind === "country") {
      contextGeographyGeneration.current += 1;
      setGeographyContext(undefined);
    }
    setContext(next);
    if (next.kind === "building" || next.kind === "transaction") syncLocalGeography(next);
    if (next.kind === "transaction") {
      if (next.mode !== mode) setMode(next.mode);
      window.setTimeout(() => {
        document.getElementById("market-context")?.scrollIntoView({
          behavior: "smooth", block: "start",
        });
      }, 80);
    }
  };
  const handlePanelContext = (next: MarketContext) => {
    if (next.kind === "country" && geographyContext) {
      setContext(geographyContext);
      return;
    }
    handleContext(next);
  };

  return (
    <div className={`app-shell ${loading ? "is-loading" : ""}`}>
      <header className="topbar">
        <button className="mobile-menu" aria-label="Odpri navigacijo"><Menu /></button>
        <Brand />
        <nav aria-label="Glavna navigacija">
          <a href="#overview" className="active">Pregled trga</a>
          <a href="#trends">Gibanje</a>
          <a href="#distribution">Porazdelitev</a>
        </nav>
        <div className="updated">
          <RefreshCw size={14} className={loading ? "spin" : ""} />
          <span>Vir: GURS ETN in RPE · do {years.to}</span>
          <i className={error ? "offline" : "live"} />
          <small>{error ? "API ni dosegljiv" : "podatki iz API"}</small>
        </div>
      </header>
      <main id="overview">
        <div className="page-intro">
          <div><h1>Analiza nepremičninskega trga</h1>
            <p>Primerljive prodaje in najemi po Sloveniji, območjih, stavbah in poslih.</p>
          </div>
        </div>
        {error && (
          <div className="dashboard-error" role="alert">
            Podatkov ni bilo mogoče osvežiti ({error}). Prikazan je zadnji uspešen rezultat;
            predstavitveni podatki se ne uporabljajo.
          </div>
        )}
        <FilterBar
          mode={mode}
          onModeChange={(next) => {
            setMode(next);
            if (next === "rent" && propertyGroup === "land") setPropertyGroup("apartment");
          }}
          propertyGroup={propertyGroup}
          onPropertyChange={setPropertyGroup}
          geographyId={geographyId}
          onGeographyChange={(id) => {
            const geography = data.geographies.find((item) => item.id === id);
            handleContext(geography
              ? { kind: "geography", id: geography.id, level: geography.level, name: geography.name }
              : { kind: "country" });
          }}
          geographies={data.geographies}
          years={years}
          onYearsChange={setYears}
          area={area}
          onAreaChange={setArea}
        />
        <div className="primary-grid contextual">
          <SloveniaMap
            mode={mode}
            context={context}
            propertyGroup={propertyGroup}
            clusters={data.clusters}
            regions={data.geographies}
            selectedId={geographyId}
            years={years}
            area={area}
            onContextChange={handleContext}
          />
          <div className="context-column" id="market-context">
            <SummaryPanel title={selectedName} mode={mode} overview={data.overview} series={data.timeSeries} />
            <ContextPanel
              context={context}
              mode={mode}
              propertyGroup={propertyGroup}
              years={years}
              overview={data.overview}
              onChange={handlePanelContext}
            />
          </div>
        </div>
        <div className="chart-grid" id="trends">
          <TrendChart mode={mode} points={data.timeSeries} years={years}
            propertyGroup={propertyGroup} geographyId={geographyId} area={area} />
          <DistributionChart mode={mode} data={data.distribution} />
        </div>
        <AnalyticsWorkspace
          mode={mode}
          propertyGroup={propertyGroup}
          years={years}
          context={analysisContext}
          overview={data.overview}
          onContextChange={handleContext}
        />
        <footer className="legal-footer">
          <div>
            <strong>Viri, licenca in odgovornost</strong>
            <p>
              Vir podatkov: Geodetska uprava Republike Slovenije (GURS), Evidenca trga
              nepremičnin (ETN) in Register prostorskih enot (RPE). Prikazano obdobje ETN:
              {` ${years.from}–${years.to}`}; stanje je vezano na zadnji uspešni lokalni uvoz.
              Podatki so obdelani in uporabljeni pod licenco{" "}
              <a href="https://creativecommons.org/licenses/by/4.0/deed.sl" target="_blank" rel="noreferrer">
                CC BY 4.0
              </a>.
            </p>
            <p>
              GURS te aplikacije ne podpira, ne potrjuje in ne odgovarja za izračune ali
              interpretacijo. Podatki GURS, izračuni aplikacije in statistične modelske ocene
              so v uporabniškem vmesniku označeni ločeno.
            </p>
            <p>
              Rezultati so informativni in niso uradna ali certificirana cenitev. Niso namenjeni
              odločanju o zavarovanem posojanju, sodnim, davčnim ali računovodskim postopkom brez
              presoje ustrezno usposobljenega strokovnjaka. Manjkajoči, napačno poročani ali
              netipični posli lahko vplivajo na rezultate.
            </p>
            <p>
              Javni ETN izvlečki ne vsebujejo imen pogodbenih strank. Aplikacija ne uvaža ali
              hrani prostih besedilnih opomb iz ETN, s čimer zmanjšuje tveganje nenamerne hrambe
              osebnih podatkov.
            </p>
          </div>
          <nav aria-label="Pravne in metodološke informacije">
            <a href="https://www.e-prostor.gov.si/dostopi/javni-dostop/" target="_blank" rel="noreferrer">
              Pogoji GURS
            </a>
            <a href="https://github.com/matkopolo/NepremicninskiAgent/blob/main/docs/data-licensing-privacy.md"
              target="_blank" rel="noreferrer">Podatki in zasebnost</a>
            <a href="https://github.com/matkopolo/NepremicninskiAgent/blob/main/docs/statistical-methodology.md"
              target="_blank" rel="noreferrer">Metodologija</a>
          </nav>
        </footer>
      </main>
    </div>
  );
}
