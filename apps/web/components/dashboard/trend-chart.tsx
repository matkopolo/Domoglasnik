import type { MarketMode, TimePoint } from "@/lib/types";

interface TrendChartProps { mode: MarketMode; points: TimePoint[] }

export function TrendChart({ mode, points }: TrendChartProps) {
  const values = points.map((point) => Number(
    mode === "sale"
      ? point.medianPricePerSqm ?? point.medianMonthlyRentPerSqm ?? 0
      : point.medianMonthlyRentPerSqm ?? point.medianPricePerSqm ?? 0,
  ));
  const min = Math.min(...values) * 0.92;
  const max = Math.max(...values) * 1.06;
  const coords = values.map((value, index) => {
    const x = 36 + (index / Math.max(values.length - 1, 1)) * 664;
    const y = 175 - ((value - min) / Math.max(max - min, 1)) * 130;
    return { x, y, value, label: points[index].period.slice(0, 4) };
  });
  const line = coords.map((point) => `${point.x},${point.y}`).join(" ");
  return (
    <section className="chart-panel trend-panel" aria-labelledby="trend-title">
      <header className="panel-heading">
        <div><h2 id="trend-title">Gibanje cen</h2><p>Mediana {mode === "sale" ? "cene" : "najemnine"} / m²</p></div>
        <span className={`chart-accent ${mode}`}>2018–2025</span>
      </header>
      <svg viewBox="0 0 736 210" role="img" aria-label="Časovni graf mediane na kvadratni meter">
        {[45, 88, 131, 175].map((y) => <line key={y} x1="36" x2="700" y1={y} y2={y} className="grid-line" />)}
        <polyline points={line} className={`trend-line ${mode}`} />
        {coords.map((point) => (
          <g key={point.label}>
            <circle cx={point.x} cy={point.y} r="4" className={`trend-dot ${mode}`} />
            <text x={point.x} y="201" textAnchor="middle" className="axis-label">{point.label}</text>
          </g>
        ))}
      </svg>
    </section>
  );
}
