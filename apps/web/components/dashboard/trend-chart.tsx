"use client";

import { RefreshCw } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { loadDetailedTimeSeries, loadInflation } from "@/lib/api";
import type { AreaRange, InflationPoint, MarketMode, PropertyGroup, TimePoint, YearRange } from "@/lib/types";
import { Tabs } from "@/components/ui/tabs";

type Tab = "price" | "volume" | "inflation";
type Interval = "month" | "quarter" | "year";
const tabs = [{ id: "price" as const, label: "Cena" }, { id: "volume" as const, label: "Število poslov" }, { id: "inflation" as const, label: "Inflacija" }];

function periodLabel(period: string, interval: Interval) {
  const [year, month] = period.split("-");
  if (interval === "year") return year;
  if (interval === "quarter") return `${year} Q${Math.floor((Number(month) - 1) / 3) + 1}`;
  return `${month}/${year.slice(2)}`;
}

export function TrendChart({ mode, points, years, propertyGroup, geographyId, area = {} }: {
  mode: MarketMode; points: TimePoint[]; years: YearRange; propertyGroup: PropertyGroup;
  geographyId?: number; area?: AreaRange;
}) {
  const [tab, setTab] = useState<Tab>("price");
  const [interval, setInterval] = useState<Interval>("year");
  const [series, setSeries] = useState(points);
  const [inflation, setInflation] = useState<InflationPoint[]>([]);
  const [activeIndex, setActiveIndex] = useState<number>();
  const [pinnedIndex, setPinnedIndex] = useState<number>();
  const [loading, setLoading] = useState(false);
  const [sourceError, setSourceError] = useState(false);
  const [inflationError, setInflationError] = useState(false);
  const generation = useRef(0);

  useEffect(() => {
    const controller = new AbortController();
    const request = ++generation.current;
    queueMicrotask(() => setLoading(true));
    loadDetailedTimeSeries(mode, propertyGroup, geographyId, years, interval, controller.signal, area).then((market) => {
      if (request === generation.current) { setSeries(market.items); setSourceError(false); }
    }).catch(() => { if (!controller.signal.aborted && request === generation.current) setSourceError(true); })
      .finally(() => { if (!controller.signal.aborted && request === generation.current) setLoading(false); });
    return () => controller.abort();
  }, [area, geographyId, interval, mode, propertyGroup, years]);

  useEffect(() => {
    if (tab !== "inflation") return;
    const controller = new AbortController();
    loadInflation(years, interval, controller.signal).then((result) => { setInflation(result.items); setInflationError(false); })
      .catch(() => { if (!controller.signal.aborted) setInflationError(true); });
    return () => controller.abort();
  }, [interval, tab, years]);

  const chartPoints = useMemo(() => tab === "inflation"
    ? inflation.map((item) => ({ period: item.period, value: item.annualRatePercent, publishable: true, incomplete: false, source: undefined as TimePoint | undefined }))
    : series.map((item) => ({ period: item.period, value: tab === "volume" ? item.transactionCount : mode === "sale" ? item.medianPricePerSqm ?? null : item.medianMonthlyRent ?? null, publishable: tab === "volume" || item.isPublishable !== false, incomplete: Boolean(item.incomplete), source: item })), [inflation, mode, series, tab]);
  const available = chartPoints.filter((point) => point.publishable && point.value != null);
  const values = available.map((point) => point.value as number);
  const rawMin = values.length ? Math.min(...values) : 0;
  const rawMax = values.length ? Math.max(...values) : 1;
  const min = tab === "volume" ? 0 : rawMin - Math.max((rawMax - rawMin) * .12, Math.abs(rawMin) * .04);
  const max = rawMax + Math.max((rawMax - rawMin) * .12, Math.abs(rawMax) * .04, 1);
  const coords = chartPoints.map((point, index) => ({ ...point, index, x: 78 + index / Math.max(chartPoints.length - 1, 1) * 622, y: point.value == null ? 175 : 175 - ((point.value - min) / Math.max(max - min, 1)) * 130, label: periodLabel(point.period, interval) }));
  const segments: string[][] = [];
  coords.forEach((point) => {
    if (!point.publishable || point.value == null) { if (segments.at(-1)?.length) segments.push([]); return; }
    if (!segments.length) segments.push([]);
    segments.at(-1)!.push(`${point.x},${point.y}`);
  });
  const grid = [45, 88, 131, 175].map((y, index) => ({ y, value: max - index / 3 * (max - min) }));
  const format = new Intl.NumberFormat("sl-SI", { maximumFractionDigits: tab === "price" && mode === "sale" || tab === "volume" ? 0 : 1 });
  const unit = tab === "price" ? mode === "sale" ? "€/m²" : "€/mesec" : tab === "inflation" ? "%" : "poslov";
  const selectedIndex = activeIndex ?? pinnedIndex;
  const selected = selectedIndex == null ? undefined : coords[selectedIndex];
  const labelStep = Math.max(1, Math.ceil(coords.length / 8));

  return <section className="chart-panel trend-panel" aria-labelledby="trend-title" aria-busy={loading}>
    <header className="panel-heading trend-heading"><div><h2 id="trend-title">Gibanje trga</h2><p>{tab === "price" ? mode === "sale" ? "Mediana cene / m²" : "Mediana mesečne najemnine" : tab === "volume" ? `Vsi evidentirani ${mode === "sale" ? "prodajni" : "najemni"} posli` : "Letna inflacija · SURS · Slovenija"}</p></div><div>{loading && <span className="localized-loading" role="status"><RefreshCw aria-hidden="true"/> Osvežujem …</span>}<span className="chart-period">{years.from}–{years.to}</span></div></header>
    <div className="trend-controls"><Tabs label="trend-metric" tabs={tabs} active={tab} onChange={(next) => { setTab(next); setPinnedIndex(undefined); }}><span className="sr-only">Izbrani prikaz: {tabs.find((item) => item.id === tab)?.label}</span></Tabs>
      <div className="frequency-picker" role="group" aria-label="Časovna frekvenca">{(["month", "quarter", "year"] as Interval[]).map((item) => <button key={item} type="button" className={interval === item ? "active" : ""} onClick={() => setInterval(item)}>{{ month: "Mesec", quarter: "Četrtletje", year: "Leto" }[item]}</button>)}</div></div>
    {sourceError && <p className="chart-source-error" role="alert">Časovne vrste trenutno ni mogoče osvežiti. Prikazani so zadnji uspešni podatki.</p>}
    {tab === "inflation" && inflationError && <p className="chart-source-error" role="alert">Vir SURS trenutno ni dosegljiv.</p>}
    {available.length === 0 ? <p className="chart-empty">Za izbrano obdobje ni podatkov.</p> : <svg viewBox="0 0 736 220" role="img" aria-label={`${tabs.find((item) => item.id === tab)?.label} po času`}>
      <text x="14" y="112" textAnchor="middle" className="axis-title" transform="rotate(-90 14 112)">{unit}</text><text x="386" y="217" textAnchor="middle" className="axis-title">Obdobje</text>
      {grid.map(({ y, value }) => <g key={y}><line x1="72" x2="700" y1={y} y2={y} className="grid-line"/><text x="63" y={y + 4} textAnchor="end" className="axis-value">{format.format(value)} {unit === "poslov" ? "" : unit}</text></g>)}
      {tab === "volume" ? coords.map((point, index) => point.value != null && <rect key={point.period}
        className={`volume-bar interactive ${point.incomplete ? "incomplete" : ""}`}
        x={point.x - Math.min(13, 260 / Math.max(coords.length, 1))} y={point.y}
        width={Math.min(26, 520 / Math.max(coords.length, 1))} height={175 - point.y}
        tabIndex={0} role="button" aria-pressed={pinnedIndex === index}
        aria-label={`${point.label}${point.incomplete ? ", nepopolno obdobje" : ""}: ${format.format(point.value)} poslov. Pritisnite za pripenjanje podrobnosti.`}
        onClick={() => setPinnedIndex((current) => current === index ? undefined : index)}
        onMouseEnter={() => setActiveIndex(index)} onMouseLeave={() => setActiveIndex(undefined)}
        onFocus={() => setActiveIndex(index)} onBlur={() => setActiveIndex(undefined)}/>)
        : segments.filter((segment) => segment.length).map((segment, index) => <polyline key={index} points={segment.join(" ")} className={`trend-line ${tab}`}/>)}
      {coords.map((point, index) => <g key={point.period}>{tab !== "volume" && point.publishable && point.value != null && <circle cx={point.x} cy={point.y} r={selectedIndex === index ? 7 : 5} className={`trend-dot ${tab} ${point.incomplete ? "incomplete" : ""}`} tabIndex={0} role="button" aria-pressed={pinnedIndex === index} aria-label={`${point.label}${point.incomplete ? ", nepopolno obdobje" : ""}: ${format.format(point.value)} ${unit}. Pritisnite za pripenjanje podrobnosti.`} onClick={() => setPinnedIndex((current) => current === index ? undefined : index)} onMouseEnter={() => setActiveIndex(index)} onMouseLeave={() => setActiveIndex(undefined)} onFocus={() => setActiveIndex(index)} onBlur={() => setActiveIndex(undefined)}/>} {index % labelStep === 0 && <text x={point.x} y="196" textAnchor="middle" className="axis-label">{point.label}{point.incomplete ? "*" : ""}</text>}</g>)}
      {selected?.publishable && selected.value != null && <g className="chart-tooltip" transform={`translate(${Math.min(625, Math.max(150, selected.x))} ${Math.max(58, selected.y - 10)})`}><rect x="-112" y="-54" width="224" height={selected.source ? 49 : 32} rx="6"/><text x="0" y="-35" textAnchor="middle"><tspan>{selected.label} · {format.format(selected.value)} {unit}</tspan>{selected.source && <><tspan x="0" dy="15">{selected.source.transactionCount} poslov · {selected.source.includedSampleSize} v vzorcu · {selected.source.excludedOutlierCount ?? 0} odstopanj</tspan></>}</text></g>}
    </svg>}
    {coords.some((point) => point.incomplete) && <p className="chart-footnote">* nepopolno obdobje</p>}
  </section>;
}
