import { ChartNoAxesColumnIncreasing } from "lucide-react";
import { suppressionReasonLabel } from "@/lib/display-values";
import type { Distribution, MarketMode } from "@/lib/types";

const format = new Intl.NumberFormat("sl-SI", { maximumFractionDigits: 0 });

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
        {data.buckets.map((bucket) => (
          <span key={bucket.index} className={mode} style={{ height: `${Math.max(10, bucket.count / max * 100)}%` }} title={`${bucket.count} poslov`} />
        ))}
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
