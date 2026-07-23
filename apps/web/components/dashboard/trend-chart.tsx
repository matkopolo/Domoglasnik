"use client";

import { useState } from "react";
import type { MarketMode, TimePoint, YearRange } from "@/lib/types";

interface TrendChartProps { mode: MarketMode; points: TimePoint[]; years?: YearRange }

export function TrendChart({ mode, points, years }: TrendChartProps) {
  const [hoveredIndex, setHoveredIndex] = useState<number>();
  const values = points.map((point) => Number(
    mode === "sale"
      ? point.medianPricePerSqm ?? point.medianMonthlyRentPerSqm ?? 0
      : point.medianMonthlyRentPerSqm ?? point.medianPricePerSqm ?? 0,
  ));
  const min = Math.min(...values) * 0.92;
  const max = Math.max(...values) * 1.06;
  const coords = values.map((value, index) => {
    const x = 78 + (index / Math.max(values.length - 1, 1)) * 622;
    const y = 175 - ((value - min) / Math.max(max - min, 1)) * 130;
    return { x, y, value, label: points[index].period.slice(0, 4) };
  });
  const line = coords.map((point) => `${point.x},${point.y}`).join(" ");
  const grid = [45, 88, 131, 175].map((y, index) => ({
    y,
    value: max - (index / 3) * (max - min),
  }));
  const format = new Intl.NumberFormat("sl-SI", {
    maximumFractionDigits: mode === "sale" ? 0 : 1,
  });
  const hovered = hoveredIndex === undefined ? undefined : coords[hoveredIndex];
  return (
    <section className="chart-panel trend-panel" aria-labelledby="trend-title">
      <header className="panel-heading">
        <div><h2 id="trend-title">Gibanje cen</h2><p>Mediana {mode === "sale" ? "cene" : "najemnine"} / m²</p></div>
        <span className={`chart-accent ${mode}`}>
          {years ? `${years.from}–${years.to}` : points.length ? `${points[0].period.slice(0, 4)}–${points.at(-1)?.period.slice(0, 4)}` : "Ni podatkov"}
        </span>
      </header>
      <svg viewBox="0 0 736 210" role="img" aria-label="Časovni graf mediane na kvadratni meter">
        {grid.map(({ y, value }) => (
          <g key={y}>
            <line x1="72" x2="700" y1={y} y2={y} className="grid-line" />
            <text x="63" y={y + 4} textAnchor="end" className="axis-value">
              {format.format(value)} €
            </text>
          </g>
        ))}
        <polyline points={line} className={`trend-line ${mode}`} />
        {coords.map((point, index) => (
          <g key={point.label}>
            <circle
              cx={point.x}
              cy={point.y}
              r={hoveredIndex === index ? 6 : 4}
              className={`trend-dot ${mode}`}
              tabIndex={0}
              role="button"
              aria-label={`${point.label}: ${format.format(point.value)} evrov na kvadratni meter`}
              onMouseEnter={() => setHoveredIndex(index)}
              onMouseLeave={() => setHoveredIndex(undefined)}
              onFocus={() => setHoveredIndex(index)}
              onBlur={() => setHoveredIndex(undefined)}
            />
            <text x={point.x} y="201" textAnchor="middle" className="axis-label">{point.label}</text>
          </g>
        ))}
        {hovered && (
          <g
            className="chart-tooltip"
            transform={`translate(${Math.min(650, Math.max(130, hovered.x))} ${Math.max(38, hovered.y - 12)})`}
          >
            <rect x="-67" y="-31" width="134" height="27" rx="5" />
            <text x="0" y="-13" textAnchor="middle">
              {hovered.label} · {format.format(hovered.value)} €/m²
            </text>
          </g>
        )}
      </svg>
    </section>
  );
}
