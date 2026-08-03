import { ChartNoAxesColumnIncreasing } from "lucide-react";
import { suppressionReasonLabel } from "@/lib/display-values";
import type { Distribution, MarketMode } from "@/lib/types";

const format = new Intl.NumberFormat("sl-SI", { maximumFractionDigits: 0 });

function transactionCountLabel(count: number) {
  if (count === 1) return "1 posel";
  if (count === 2) return "2 posla";
  if (count === 3 || count === 4) return `${count} posli`;
  return `${count.toLocaleString("sl-SI")} poslov`;
}

export function DistributionChart({ data, mode }: { data: Distribution; mode: MarketMode }) {
  const max = Math.max(...data.buckets.map((bucket) => bucket.count), 1);
  const percentiles = data.percentiles;
  return (
    <section id="distribution" className="chart-panel distribution-panel" aria-labelledby="distribution-title">
      <header className="panel-heading">
        <div><h2 id="distribution-title">Porazdelitev cen</h2><p>{data.sampleSize.toLocaleString("sl-SI")} primerljivih poslov</p></div>
      </header>
      {!data.isPublishable || !percentiles ? (
        <div className="suppressed-statistic" role="status">
          <span className="suppressed-statistic-icon"><ChartNoAxesColumnIncreasing size={22} /></span>
          <div>
            <strong>Za porazdelitev je premalo podatkov</strong>
            <p>{suppressionReasonLabel(data.suppressionReason, data.sampleSize)}</p>
            <small>Ko bo prag dosežen, se histogram prikaže samodejno.</small>
          </div>
        </div>
      ) : <>
      <div className="histogram" aria-label={mode === "sale"
        ? "Histogram vrednosti na kvadratni meter"
        : "Histogram mesečnih najemnin"}>
        <span className="histogram-y-axis">Število poslov</span>
        {data.buckets.map((bucket) => {
          const unit = mode === "sale" ? "€/m²" : "€/mesec";
          const label = `${format.format(bucket.from)}–${format.format(bucket.to)} ${unit}: ${transactionCountLabel(bucket.count)}`;
          return (
            <button key={bucket.index} type="button" className={`histogram-bar ${mode}`}
              style={{ height: `${Math.max(10, bucket.count / max * 100)}%` }} aria-label={label}>
              <span className="histogram-tooltip" role="tooltip">
                <strong>{transactionCountLabel(bucket.count)}</strong>
                <small>{format.format(bucket.from)}–{format.format(bucket.to)} {unit}</small>
              </span>
            </button>
          );
        })}
        <span className="histogram-x-axis">{mode === "sale" ? "Cena na m²" : "Mesečna najemnina"}</span>
      </div>
      <div className="percentile-track">
        {(["p10", "p25", "p50", "p75", "p90"] as const).map((key) => (
          <div key={key} className={key === "p50" ? `median ${mode}` : ""}>
            <span>{key === "p50" ? "Mediana" : `${key.slice(1)}. percentil`}</span>
            <strong>{format.format(percentiles[key])} €</strong>
          </div>
        ))}
      </div>
      </>}
    </section>
  );
}
