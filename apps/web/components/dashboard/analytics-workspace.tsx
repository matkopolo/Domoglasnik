"use client";

import {
  AlertTriangle,
  BarChart3,
  BrainCircuit,
  MapPinned,
  TrendingUp,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import {
  loadAnomalySummary,
  loadDetailedTimeSeries,
  loadGeography,
  loadGeographyChildren,
  loadGeographyRankings,
  loadMarketIndices,
  loadStatisticalOutliers,
  loadTransactionExtremes,
} from "@/lib/api";
import { decimal, euro, indexChangeInterpretation, propertyGroupLabel, suppressionReasonLabel } from "@/lib/display-values";
import type {
  AnomalySummary,
  GeographyComparison,
  MarketContext,
  MarketIndex,
  MarketMode,
  Overview,
  PropertyGroup,
  StatisticalOutlier,
  TimePoint,
  TransactionExtreme,
  YearRange,
} from "@/lib/types";

type WorkspaceTab = "market" | "indices" | "geographies" | "outliers";
type Interval = "month" | "quarter" | "year";
type WorkspacePayload = {
  points?: TimePoint[]; extremes?: TransactionExtreme[]; indices?: MarketIndex[];
  rankings?: GeographyComparison[]; outliers?: StatisticalOutlier[];
  outlierTotal?: number; anomalySummary?: AnomalySummary;
};

interface AnalyticsWorkspaceProps {
  mode: MarketMode;
  propertyGroup: PropertyGroup;
  years: YearRange;
  context: MarketContext;
  overview: Overview;
  onContextChange: (context: MarketContext) => void;
}

function percentage(value: number | undefined) {
  return value == null || !Number.isFinite(value) ? "—" : `${decimal.format(value)} %`;
}

function growth(points: TimePoint[], mode: MarketMode) {
  const values = points
    .map((point) => mode === "sale" ? point.medianPricePerSqm : point.medianMonthlyRent)
    .filter((value): value is number => value != null);
  return values.length < 2 || values[0] <= 0 ? undefined : 100 * (values.at(-1)! / values[0] - 1);
}

function MarketDepth({
  overview,
  points,
  mode,
  extremes,
  onContextChange,
}: {
  overview: Overview;
  points: TimePoint[];
  mode: MarketMode;
  extremes: TransactionExtreme[];
  onContextChange: (context: MarketContext) => void;
}) {
  const representativeRate = overview.transactionCount > 0
    ? 100 * overview.includedSampleSize / overview.transactionCount
    : undefined;
  const outlierRate = overview.eligibleCount > 0
    ? 100 * overview.excludedOutlierCount / overview.eligibleCount
    : undefined;
  const periodGrowth = growth(points, mode);
  const median = mode === "sale" ? overview.medianPrice : overview.medianMonthlyRent;
  const cleaned = overview.cleanedAveragePrice;
  const raw = overview.rawAveragePrice;
  return (
    <>
      <div className="analytics-kpis">
        <article><span>Vsi evidentirani posli</span><strong>
          {overview.transactionCount.toLocaleString("sl-SI")}
        </strong><small>v aktivnem območju in obdobju</small></article>
        <article><span>Mediana posla</span><strong>{median == null ? "—" : euro.format(median)}</strong>
          <small>robustna sredina reprezentativnega vzorca</small></article>
        <article><span>Mediana površine</span><strong>
          {overview.medianArea == null ? "—" : `${decimal.format(overview.medianArea)} m²`}
        </strong><small>primerljiva referenčna površina</small></article>
        <article><span>Reprezentativni delež</span><strong>{percentage(representativeRate)}</strong>
          <small>{overview.includedSampleSize.toLocaleString("sl-SI")} od {overview.transactionCount.toLocaleString("sl-SI")} poslov</small></article>
        <article><span>Statistična odstopanja</span><strong>{percentage(outlierRate)}</strong>
          <small>{overview.excludedOutlierCount.toLocaleString("sl-SI")} izločenih iz reprezentativne statistike</small></article>
        <article><span>Sprememba obdobja</span><strong className="direction-neutral">
          {periodGrowth == null ? "—" : `${periodGrowth > 0 ? "+" : ""}${decimal.format(periodGrowth)} %`}
        </strong><small>prva proti zadnji objavljeni točki</small></article>
        <article><span>Objavljivost</span><strong>{overview.isPublishable === false ? "Ne" : "Da"}</strong>
          <small>{overview.suppressionReason
            ? suppressionReasonLabel(overview.suppressionReason, overview.includedSampleSize)
            : `${overview.includedSampleSize.toLocaleString("sl-SI")} opazovanj`}</small></article>
      </div>
      <div className="extremes-grid">
        {(["highest", "lowest"] as const).map((direction) => (
          <section key={direction}>
            <header>
              <span>{mode === "sale"
                ? direction === "highest" ? "Najvišje prodaje" : "Najnižje prodaje"
                : direction === "highest" ? "Najvišje najemnine" : "Najnižje najemnine"}</span>
              <small>reprezentativna skupna cena</small>
            </header>
            {extremes.filter((item) => item.direction === direction).map((item) => (
              <button type="button" key={`${direction}-${item.transactionId}`}
                onClick={() => onContextChange({
                  kind: "transaction", mode, id: item.transactionId,
                })}>
                <strong>#{item.transactionId}</strong>
                <span>{item.amount == null ? "—" : euro.format(item.amount)}</span>
                <small>
                  {decimal.format(item.amountPerSqm)} €/m²
                  {item.area == null ? "" : ` · ${decimal.format(item.area)} m²`}
                </small>
              </button>
            ))}
          </section>
        ))}
      </div>
      {mode === "sale" && (cleaned != null || raw != null) && (
        <div className="average-comparison">
          <div><span>Mediana</span><strong>{median == null ? "—" : euro.format(median)}</strong></div>
          <div><span>Očiščeno povprečje</span><strong>{cleaned == null ? "—" : euro.format(cleaned)}</strong></div>
          <div><span>Surovo povprečje</span><strong>{raw == null ? "—" : euro.format(raw)}</strong></div>
          <p>Razlika pokaže, koliko nenavadni posli premaknejo navadno povprečje. Mediana ostaja glavni opisni kazalnik.</p>
        </div>
      )}
      <div className="period-table">
        <div className="period-row header"><span>Obdobje</span><span>
          {mode === "sale" ? "Mediana / m²" : "Mesečna mediana"}
        </span><span>Vzorec</span><span>Odstopanja</span></div>
        {points.slice(-16).map((point) => {
          const value = mode === "sale" ? point.medianPricePerSqm : point.medianMonthlyRent;
          return (
            <div className="period-row" key={point.period}>
              <strong>{point.period.slice(0, 7)}{point.incomplete ? " *" : ""}</strong>
              <span>{value == null ? "—" : mode === "sale" ? `${decimal.format(value)} €/m²` : `${euro.format(value)} / mesec`}</span>
              <span>{point.includedSampleSize.toLocaleString("sl-SI")}</span>
              <span>{point.excludedOutlierCount?.toLocaleString("sl-SI") ?? "—"}</span>
            </div>
          );
        })}
      </div>
    </>
  );
}

function IndexExplorer({ items }: { items: MarketIndex[] }) {
  const [hovered, setHovered] = useState<{
    method: string;
    point: MarketIndex;
    x: number;
    y: number;
  }>();
  const groups = useMemo(() => {
    const result = new Map<string, MarketIndex[]>();
    items.forEach((item) => {
      const list = result.get(item.method) ?? [];
      list.push(item);
      result.set(item.method, list);
    });
    return result;
  }, [items]);
  const allValues = items.filter((item) => item.isPublishable).map((item) => item.indexValue);
  const min = Math.min(...allValues, 80);
  const max = Math.max(...allValues, 120);
  const colors = { hedonic: "#0d5c3d", matched_pair_median: "#c15f3c" };
  const periods = [...new Set(items.filter((item) => item.isPublishable).map((item) => item.period))].sort();
  const xForPeriod = (period: string) => 80
    + periods.indexOf(period) / Math.max(periods.length - 1, 1) * 775;
  const periodLabelStep = Math.max(1, Math.ceil(periods.length / 9));
  const hasHedonic = groups.has("hedonic");
  const methodLabel = (method: string) => method === "hedonic"
    ? "Hedonski indeks"
    : "Indeks ponovnih prodaj";
  return (
    <>
      <div className="methodology-callout">
        <BrainCircuit size={20} />
        <div><strong>Dva različna pogleda na gibanje cen</strong>
          <p>Hedonski indeks primerja cene, kot da bi se skozi čas prodajale enako kakovostne nepremičnine. Indeks ponovnih prodaj spremlja nepremičnine, ki so bile prodane večkrat. Začetno leto je označeno s 100. Nad 100 pomeni rast cen, pod 100 padec, natanko 100 pa približno nespremenjene cene. Primer: 120 pomeni približno 20 % višje, 90 pa približno 10 % nižje cene kot v začetnem letu.</p></div>
      </div>
      {!hasHedonic && items.length > 0 && <p className="chart-source-error" role="status">
        Hedonski indeks še ni izračunan. Prikazan je samo indeks ponovnih prodaj; hedonska serija bo na voljo po končanem učenju modela.
      </p>}
      {items.length === 0 ? <p>Za izbrano vrsto ni izračunanega indeksa.</p> : (
        <svg className="index-chart" viewBox="0 0 900 300" role="img" aria-label="Primerjava tržnih indeksov">
          <text x="16" y="145" textAnchor="middle" className="axis-title"
            transform="rotate(-90 16 145)">Raven cen (začetno leto = 100)</text>
          <text x="475" y="292" textAnchor="middle" className="axis-title">Leto</text>
          {[0, 1, 2, 3].map((line) => {
            const value = max - line * (max - min) / 3;
            const y = 35 + line * 70;
            return <g key={line}><line x1="70" x2="870" y1={y} y2={y} className="grid-line" />
              <text x="60" y={y + 4} textAnchor="end" className="axis-value">{decimal.format(value)}</text></g>;
          })}
          {[...groups.entries()].map(([method, points]) => {
            const published = points.filter((point) => point.isPublishable);
            const coords = published.map((point) => ({
              x: xForPeriod(point.period),
              y: 245 - (point.indexValue - min) / Math.max(max - min, 1) * 210,
              point,
            }));
            return <g key={method}>
              <polyline fill="none" stroke={colors[method as keyof typeof colors]} strokeWidth="3"
                points={coords.map((point) => `${point.x},${point.y}`).join(" ")} />
              {coords.map(({ x, y, point }) => <circle key={point.period} cx={x} cy={y}
                r={hovered?.method === method && hovered.point.period === point.period ? "6" : "4"}
                fill={colors[method as keyof typeof colors]} className="index-point" tabIndex={0} role="button"
                aria-label={`${methodLabel(method)}, ${point.period.slice(0, 7)}: indeks ${decimal.format(point.indexValue)}, ${indexChangeInterpretation(point.indexValue, point.basePeriod)}, ${point.sampleSize} poslov`}
                onMouseEnter={() => setHovered({ method, point, x, y })}
                onMouseLeave={() => setHovered(undefined)}
                onFocus={() => setHovered({ method, point, x, y })}
                onBlur={() => setHovered(undefined)} />)}
            </g>;
          })}
          {periods.filter((_, index) => index % periodLabelStep === 0 || index === periods.length - 1)
            .map((period) => <text key={period} x={xForPeriod(period)} y="272" textAnchor="middle"
              className="axis-label">{period.slice(0, 4)}</text>)}
          {hovered && <g className="chart-tooltip"
            transform={`translate(${Math.min(745, Math.max(175, hovered.x))} ${Math.max(58, hovered.y - 9)})`}>
            <rect x="-165" y="-49" width="330" height="42" rx="6" />
            <text x="0" y="-32" textAnchor="middle">
              {methodLabel(hovered.method)} · {hovered.point.period.slice(0, 7)} · indeks {decimal.format(hovered.point.indexValue)}
            </text>
            <text x="0" y="-17" textAnchor="middle">
              {indexChangeInterpretation(hovered.point.indexValue, hovered.point.basePeriod)} · n={hovered.point.sampleSize}
            </text>
          </g>}
        </svg>
      )}
      <div className="index-legend">
        <span><i className="hedonic" /> Hedonski indeks</span>
        <span><i className="matched" /> Indeks ponovnih prodaj</span>
      </div>
      <div className="index-latest-grid">
        {[...groups.entries()].map(([method, points]) => {
          const latest = points.filter((point) => point.isPublishable).at(-1);
          if (!latest) return null;
          return <article key={method}>
            <span>{method === "hedonic" ? "Hedonski" : "Ponovne prodaje"}</span>
            <strong>{indexChangeInterpretation(latest.indexValue, latest.basePeriod)}</strong>
            <small>indeks {decimal.format(latest.indexValue)} · {latest.period.slice(0, 7)} · n={latest.sampleSize}
              {latest.lowerIndexValue != null && latest.upperIndexValue != null
                ? ` · interval ${decimal.format(latest.lowerIndexValue)}–${decimal.format(latest.upperIndexValue)}`
                : ""}</small>
          </article>;
        })}
      </div>
    </>
  );
}

function GeographyExplorer({
  rankings,
  mode,
  onContextChange,
}: {
  rankings: GeographyComparison[];
  mode: MarketMode;
  onContextChange: (context: MarketContext) => void;
}) {
  const amount = (item: GeographyComparison) => mode === "rent"
    ? item.medianAmount
    : item.medianAmountPerSqm;
  const sorted = [...rankings].sort((a, b) => (amount(b) ?? -1) - (amount(a) ?? -1));
  const max = Math.max(...sorted.map((item) => amount(item) ?? 0), 1);
  const showOnMap = () => {
    document.querySelector<HTMLElement>(".map-panel")?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  };
  const selectGeography = (item: GeographyComparison) => {
    onContextChange({ kind: "geography", id: item.id, level: item.level, name: item.name });
    window.setTimeout(showOnMap, 80);
  };
  return (
    <>
      <div className="geography-navigation" aria-label="Navigacija primerjave območij">
        <button type="button" className="map-return" onClick={showOnMap}>
          <MapPinned size={14} /> Prikaži na zemljevidu
        </button>
      </div>
      <div className="ranking-table">
        <div className="ranking-row header"><span>#</span><span>Območje</span><span>
          {mode === "sale" ? "Mediana / m²" : "Mesečna mediana"}
        </span><span>Vsi posli</span><span>V vzorcu</span></div>
        {sorted.map((item, index) => (
          <button type="button" className="ranking-row" key={item.id} onClick={() => selectGeography(item)}>
            <b>{index + 1}</b><strong>{item.name}</strong>
            <span className="ranking-value">
              <i style={{ width: `${100 * (amount(item) ?? 0) / max}%` }} />
              {amount(item) == null ? "—" : euro.format(amount(item)!)}
            </span>
            <span>{item.transactionCount.toLocaleString("sl-SI")}</span>
            <span>{item.representativeCount.toLocaleString("sl-SI")}</span>
          </button>
        ))}
        {sorted.length === 0 && <p>Za to raven ni primerljivih območij.</p>}
        <small>{mode === "sale" ? "Prodajne cene" : "Mesečne najemnine"}; razvrščeno po reprezentativni mediani.</small>
      </div>
    </>
  );
}

function OutlierExplorer({
  items,
  total,
  summary,
  mode,
  onContextChange,
}: {
  items: StatisticalOutlier[];
  total: number;
  summary?: AnomalySummary;
  mode: MarketMode;
  onContextChange: (context: MarketContext) => void;
}) {
  return (
    <>
      <div className="anomaly-summary-grid">
        <article><span>Robustna statistična odstopanja</span><strong>{total.toLocaleString("sl-SI")}</strong></article>
        <article><span>Vsi ocenjeni signali</span><strong>{summary?.total.toLocaleString("sl-SI") ?? "—"}</strong></article>
        <article><span>Označeni signali</span><strong>{summary?.flagged.toLocaleString("sl-SI") ?? "—"}</strong></article>
        <article><span>Visoka prioriteta</span><strong>{summary?.highPriority.toLocaleString("sl-SI") ?? "—"}</strong></article>
      </div>
      <div className="methodology-callout warning">
        <AlertTriangle size={20} /><div><strong>Odstopanje ni nujno napaka</strong>
          <p>To je pregledni seznam nenavadnih poslov. Posli niso samodejno izbrisani; reprezentativne statistike uporabljajo potrjena pravila kakovosti.</p></div>
      </div>
      <div className="outlier-table">
        <div className="outlier-row header"><span>Posel</span><span>Vrednost / m²</span><span>Mediana kohorte</span><span>MAD z</span><span>n</span></div>
        {items.map((item) => {
          const value = mode === "sale" ? item.pricePerSqm : item.monthlyRentPerSqm;
          return <button type="button" className="outlier-row" key={item.transactionId}
            onClick={() => onContextChange({ kind: "transaction", mode, id: item.transactionId })}>
            <strong>#{item.transactionId}<small>{item.contractDate}</small></strong>
            <span>{value == null ? "—" : `${decimal.format(value)} ${mode === "sale" ? "€/m²" : "€/m² na mesec"}`}</span>
            <span>{decimal.format(item.cohortMedian)} {mode === "sale" ? "€/m²" : "€/m² na mesec"}</span>
            <span>{item.modifiedZScore == null ? "—" : decimal.format(item.modifiedZScore)}</span>
            <span>{item.cohortSize}</span>
          </button>;
        })}
      </div>
    </>
  );
}

export function AnalyticsWorkspace({
  mode,
  propertyGroup,
  years,
  context,
  overview,
  onContextChange,
}: AnalyticsWorkspaceProps) {
  const [tab, setTab] = useState<WorkspaceTab>("market");
  const [interval, setInterval] = useState<Interval>("quarter");
  const [points, setPoints] = useState<TimePoint[]>([]);
  const [indices, setIndices] = useState<MarketIndex[]>([]);
  const [rankings, setRankings] = useState<GeographyComparison[]>([]);
  const [outliers, setOutliers] = useState<StatisticalOutlier[]>([]);
  const [outlierTotal, setOutlierTotal] = useState(0);
  const [anomalySummary, setAnomalySummary] = useState<AnomalySummary>();
  const [extremes, setExtremes] = useState<TransactionExtreme[]>([]);
  const [loading, setLoading] = useState(false);
  const generation = useRef(0);
  const cache = useRef(new Map<string, WorkspacePayload>());
  const geographyId = context.kind === "geography" ? context.id : undefined;

  useEffect(() => {
    const controller = new AbortController();
    const requestGeneration = ++generation.current;
    const cacheKey = JSON.stringify({ tab, mode, propertyGroup, geographyId, years, interval: tab === "market" ? interval : undefined, context: context.kind === "geography" ? context.id : "country" });
    const apply = (payload: WorkspacePayload) => {
      if (payload.points) setPoints(payload.points);
      if (payload.extremes) setExtremes(payload.extremes);
      if (payload.indices) setIndices(payload.indices);
      if (payload.rankings) setRankings(payload.rankings);
      if (payload.outliers) setOutliers(payload.outliers);
      if (payload.outlierTotal != null) setOutlierTotal(payload.outlierTotal);
      if (payload.anomalySummary) setAnomalySummary(payload.anomalySummary);
    };
    const cached = cache.current.get(cacheKey);
    if (cached) { apply(cached); setLoading(false); return () => controller.abort(); }
    queueMicrotask(() => { if (!controller.signal.aborted) setLoading(true); });
    let request: Promise<WorkspacePayload>;
    if (tab === "market") request = Promise.all([
      loadDetailedTimeSeries(mode, propertyGroup, geographyId, years, interval, controller.signal),
      loadTransactionExtremes(mode, propertyGroup, geographyId, years, controller.signal),
    ]).then(([series, result]) => ({ points: series.items, extremes: result.items }));
    else if (tab === "indices") request = loadMarketIndices(mode, propertyGroup, controller.signal).then((result) => ({ indices: result.items }));
    else if (tab === "outliers") request = Promise.all([
      loadStatisticalOutliers(mode, propertyGroup, geographyId, years, controller.signal), loadAnomalySummary(mode, controller.signal),
    ]).then(([result, summary]) => ({ outliers: result.items, outlierTotal: result.total, anomalySummary: summary }));
    else {
      const rankingRequest = context.kind === "country"
        ? loadGeographyRankings(mode, "statistical_region", [13.3, 45.4, 16.7, 46.9], propertyGroup, years, controller.signal)
        : context.kind === "geography" ? Promise.all([loadGeography(context.id, controller.signal), loadGeographyChildren(context.id, controller.signal)]).then(([geography, children]) => {
          if (!geography.bounds || children.items.length === 0) return [];
          return loadGeographyRankings(mode, children.items[0].level, geography.bounds, propertyGroup, years, controller.signal, geography.id);
        }) : Promise.resolve([]);
      request = rankingRequest.then((result) => ({ rankings: result }));
    }
    request.then((payload) => {
      if (requestGeneration !== generation.current) return;
      cache.current.set(cacheKey, payload);
      apply(payload);
    }).catch(() => undefined).finally(() => {
      if (!controller.signal.aborted && requestGeneration === generation.current) setLoading(false);
    });
    return () => controller.abort();
  }, [context, geographyId, interval, mode, propertyGroup, tab, years]);

  const tabOptions: Array<{ id: WorkspaceTab; label: string; icon: typeof BarChart3 }> = [
    { id: "market", label: "Tržna slika", icon: BarChart3 },
    { id: "geographies", label: "Primerjava območij", icon: MapPinned },
    { id: "indices", label: "Indeksi", icon: TrendingUp },
    { id: "outliers", label: "Kakovost podatkov", icon: AlertTriangle },
  ];
  return (
    <section id="detailed-analytics" className={`analytics-workspace ${loading ? "loading" : ""}`} aria-labelledby="analytics-title" aria-busy={loading}>
      <header className="analytics-header">
        <div><span>Podrobna analiza</span><h2 id="analytics-title">Podrobnejši pogled na izbrani trg</h2>
          <p>Tržna globina, območja, indeksi ter kakovost podatkov.</p></div>
        <div className="analytics-context">{mode === "sale" ? "Prodaja" : "Najem"} · {propertyGroupLabel(propertyGroup)} · {years.from}–{years.to}</div>
      </header>
      <Tabs label="analytics" className="analytics-tabs-container" tabs={tabOptions.map(({ id, label }) => ({ id, label }))} active={tab} onChange={setTab}>
      <div className="analytics-body">{loading && <span className="localized-loading analytics-refresh" role="status">Osvežujem izbrani pogled …</span>}
        {tab === "market" && <>
          <div className="interval-switch">
            {(["month", "quarter", "year"] as Interval[]).map((value) => <button type="button"
              className={interval === value ? "active" : ""} key={value} onClick={() => setInterval(value)}>
              {value === "month" ? "Mesec" : value === "quarter" ? "Četrtletje" : "Leto"}
            </button>)}
          </div>
          <MarketDepth overview={overview} points={points} mode={mode}
            extremes={extremes} onContextChange={onContextChange} />
        </>}
        {tab === "indices" && <IndexExplorer items={indices} />}
        {tab === "geographies" && <GeographyExplorer rankings={rankings} mode={mode} onContextChange={onContextChange} />}
        {tab === "outliers" && <OutlierExplorer items={outliers} total={outlierTotal}
          summary={anomalySummary} mode={mode} onContextChange={onContextChange} />}
      </div>
      </Tabs>
    </section>
  );
}
