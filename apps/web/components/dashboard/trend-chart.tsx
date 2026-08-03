"use client";

import { useEffect, useMemo, useState } from "react";
import { loadDetailedTimeSeries, loadInflation } from "@/lib/api";
import type {
  AreaRange, InflationPoint, MarketMode, PropertyGroup, TimePoint, YearRange,
} from "@/lib/types";

type Tab = "price" | "volume" | "inflation";
type Interval = "month" | "quarter" | "year";

interface TrendChartProps {
  mode: MarketMode;
  points: TimePoint[];
  years: YearRange;
  propertyGroup: PropertyGroup;
  geographyId?: number;
  area?: AreaRange;
}

const tabLabels: Record<Tab, string> = {
  price: "Cena",
  volume: "Število poslov",
  inflation: "Inflacija",
};

function periodLabel(period: string, interval: Interval) {
  const [year, month] = period.split("-");
  if (interval === "year") return year;
  if (interval === "quarter") return `${year} Q${Math.floor((Number(month) - 1) / 3) + 1}`;
  return `${month}/${year.slice(2)}`;
}

export function TrendChart({
  mode, points, years, propertyGroup, geographyId, area = {},
}: TrendChartProps) {
  const [tab, setTab] = useState<Tab>("price");
  const [interval, setInterval] = useState<Interval>("year");
  const [series, setSeries] = useState(points);
  const [inflation, setInflation] = useState<InflationPoint[]>([]);
  const [hoveredIndex, setHoveredIndex] = useState<number>();
  const [sourceError, setSourceError] = useState(false);
  const [inflationError, setInflationError] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    loadDetailedTimeSeries(
      mode, propertyGroup, geographyId, years, interval, controller.signal, area,
    ).then((market) => {
      setSeries(market.items);
      setSourceError(false);
    }).catch(() => {
      if (!controller.signal.aborted) setSourceError(true);
    });
    loadInflation(years, interval, controller.signal).then((prices) => {
      setInflation(prices.items);
      setInflationError(false);
    }).catch(() => {
      if (!controller.signal.aborted) setInflationError(true);
    });
    return () => controller.abort();
  }, [area, geographyId, interval, mode, propertyGroup, years]);

  const chartPoints = useMemo(() => {
    if (tab === "inflation") {
      return inflation.map((point) => ({ period: point.period, value: point.annualRatePercent }));
    }
    return series.map((point) => ({
      period: point.period,
      value: tab === "volume"
        ? point.transactionCount
        : Number(mode === "sale"
          ? point.medianPricePerSqm ?? 0
          : point.medianMonthlyRent ?? 0),
    })).filter((point) => tab === "volume" || point.value > 0);
  }, [inflation, mode, series, tab]);

  const values = chartPoints.map((point) => point.value);
  const rawMin = values.length ? Math.min(...values) : 0;
  const rawMax = values.length ? Math.max(...values) : 1;
  const min = tab === "volume" ? 0 : rawMin - Math.max((rawMax - rawMin) * 0.12, Math.abs(rawMin) * 0.04);
  const max = rawMax + Math.max((rawMax - rawMin) * 0.12, Math.abs(rawMax) * 0.04, 1);
  const coords = chartPoints.map((point, index) => ({
    x: 78 + (index / Math.max(chartPoints.length - 1, 1)) * 622,
    y: 175 - ((point.value - min) / Math.max(max - min, 1)) * 130,
    value: point.value,
    period: point.period,
    label: periodLabel(point.period, interval),
  }));
  const line = coords.map((point) => `${point.x},${point.y}`).join(" ");
  const grid = [45, 88, 131, 175].map((y, index) => ({
    y, value: max - (index / 3) * (max - min),
  }));
  const format = new Intl.NumberFormat("sl-SI", {
    maximumFractionDigits: tab === "price" && mode === "sale" || tab === "volume" ? 0 : 1,
  });
  const unit = tab === "price" ? mode === "sale" ? "€/m²" : "€/mesec" : tab === "inflation" ? "%" : "poslov";
  const hovered = hoveredIndex === undefined ? undefined : coords[hoveredIndex];
  const labelStep = Math.max(1, Math.ceil(coords.length / 8));

  return (
    <section className="chart-panel trend-panel" aria-labelledby="trend-title">
      <header className="panel-heading trend-heading">
        <div><h2 id="trend-title">Gibanje trga</h2><p>
          {tab === "price" && (mode === "sale" ? "Mediana cene / m²" : "Mediana mesečne najemnine")}
          {tab === "volume" && `Vsi evidentirani ${mode === "sale" ? "prodajni" : "najemni"} posli`}
          {tab === "inflation" && "Letna inflacija v Sloveniji · SURS H281S"}
        </p></div>
        <span className={`chart-accent ${mode}`}>{years.from}–{years.to}</span>
      </header>
      <div className="trend-controls">
        <div className="chart-tabs" role="tablist" aria-label="Kazalnik grafa">
          {(Object.keys(tabLabels) as Tab[]).map((item) => (
            <button key={item} type="button" role="tab" aria-selected={tab === item}
              className={tab === item ? "active" : ""} onClick={() => setTab(item)}>
              {tabLabels[item]}
            </button>
          ))}
        </div>
        <div className="frequency-picker" aria-label="Časovna frekvenca">
          {(["month", "quarter", "year"] as Interval[]).map((item) => (
            <button key={item} type="button" className={interval === item ? "active" : ""}
              onClick={() => setInterval(item)}>
              {{ month: "Mesec", quarter: "Četrtletje", year: "Leto" }[item]}
            </button>
          ))}
        </div>
      </div>
      {sourceError && <p className="chart-source-error">Časovne vrste trenutno ni mogoče osvežiti.</p>}
      {tab === "inflation" && inflationError && (
        <p className="chart-source-error">Vir SURS trenutno ni dosegljiv.</p>
      )}
      {coords.length === 0 ? <p className="chart-empty">Za izbrano obdobje ni podatkov.</p> : (
        <svg viewBox="0 0 736 210" role="img" aria-label={`${tabLabels[tab]} po času`}>
          {grid.map(({ y, value }) => <g key={y}>
            <line x1="72" x2="700" y1={y} y2={y} className="grid-line" />
            <text x="63" y={y + 4} textAnchor="end" className="axis-value">
              {format.format(value)} {tab === "price" ? "€" : tab === "inflation" ? "%" : ""}
            </text>
          </g>)}
          <polyline points={line} className={`trend-line ${mode} ${tab}`} />
          {coords.map((point, index) => <g key={point.period}>
            <circle cx={point.x} cy={point.y} r={hoveredIndex === index ? 6 : 4}
              className={`trend-dot ${mode} ${tab}`} tabIndex={0} role="button"
              aria-label={`${point.label}: ${format.format(point.value)} ${unit}`}
              onMouseEnter={() => setHoveredIndex(index)} onMouseLeave={() => setHoveredIndex(undefined)}
              onFocus={() => setHoveredIndex(index)} onBlur={() => setHoveredIndex(undefined)} />
            {index % labelStep === 0 && <text x={point.x} y="201" textAnchor="middle"
              className="axis-label">{point.label}</text>}
          </g>)}
          {hovered && <g className="chart-tooltip"
            transform={`translate(${Math.min(650, Math.max(130, hovered.x))} ${Math.max(38, hovered.y - 12)})`}>
            <rect x="-78" y="-31" width="156" height="27" rx="5" />
            <text x="0" y="-13" textAnchor="middle">
              {hovered.label} · {format.format(hovered.value)} {unit}
            </text>
          </g>}
        </svg>
      )}
    </section>
  );
}
