import { ArrowDownRight, ArrowUpRight, CircleHelp } from "lucide-react";
import type { MarketMode, Overview, TimePoint } from "@/lib/types";

const euro = new Intl.NumberFormat("sl-SI", { style: "currency", currency: "EUR", maximumFractionDigits: 0 });
const number = new Intl.NumberFormat("sl-SI");

interface SummaryPanelProps {
  title: string;
  mode: MarketMode;
  overview: Overview;
  series: TimePoint[];
}

export function SummaryPanel({ title, mode, overview, series }: SummaryPanelProps) {
  const metric = mode === "sale" ? overview.medianPricePerSqm : overview.medianMonthlyRentPerSqm;
  const values = series.map((point) => mode === "sale" ? point.medianPricePerSqm : point.medianMonthlyRentPerSqm).filter((value): value is number => value != null);
  const annualChange = values.length > 1 ? ((values.at(-1)! / values.at(-2)!) - 1) * 100 : 0;
  const positive = annualChange >= 0;
  return (
    <aside className={`summary-panel ${mode}`}>
      <div className="summary-title">
        <div><span>Izbrano območje</span><h2>{title}</h2></div>
        <CircleHelp size={17} aria-label="Kazalniki vključujejo le primerljive posle" />
      </div>
      <div className="primary-metric">
        <span>{mode === "sale" ? "Srednja cena / m²" : "Srednja najemnina / m²"}</span>
        <strong>{metric != null ? euro.format(metric) : "—"}</strong>
      </div>
      <div className="metric-row">
        <div><span>Letna sprememba</span><strong className={positive ? "positive" : "negative"}>
          {positive ? <ArrowUpRight size={18} /> : <ArrowDownRight size={18} />}
          {annualChange > 0 ? "+" : ""}{annualChange.toFixed(1).replace(".", ",")} %
        </strong></div>
        <div><span>Število poslov</span><strong>{number.format(overview.transactionCount)}</strong></div>
      </div>
      <div className="sample-block">
        <span>Uporabni vzorec</span>
        <strong>{number.format(overview.includedSampleSize)}</strong>
        <small>{overview.excludedOutlierCount ? `${number.format(overview.excludedOutlierCount)} statističnih odstopanj ni v povprečju` : "Preverjeni primerljivi posli"}</small>
      </div>
      <div className="summary-foot">
        <span>{mode === "sale" ? "Mediana posla" : "Mediana mesečnega najema"}</span>
        <strong>{euro.format(mode === "sale" ? overview.medianPrice ?? 0 : overview.medianMonthlyRent ?? 0)}</strong>
      </div>
    </aside>
  );
}
