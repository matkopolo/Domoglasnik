import { suppressionReasonLabel } from "@/lib/display-values";
import type { Distribution, MarketMode } from "@/lib/types";
import { SuppressedStatistic } from "@/components/ui/suppressed-statistic";

const format = new Intl.NumberFormat("sl-SI", { maximumFractionDigits: 0 });
function countLabel(count: number) { return count === 1 ? "1 posel" : count === 2 ? "2 posla" : count < 5 ? `${count} posli` : `${count.toLocaleString("sl-SI")} poslov`; }

export function DistributionChart({ data, mode }: { data: Distribution; mode: MarketMode }) {
  const max = Math.max(...data.buckets.map((bucket) => bucket.count), 1);
  const unit = mode === "sale" ? "€/m²" : "€/mesec";
  return <section id="distribution" className="chart-panel distribution-panel" aria-labelledby="distribution-title">
    <header className="panel-heading"><div><h2 id="distribution-title">Porazdelitev cen</h2><p>{data.sampleSize.toLocaleString("sl-SI")} reprezentativnih poslov</p></div></header>
    {!data.isPublishable || !data.percentiles ? <><SuppressedStatistic reason={data.suppressionReason} sampleSize={data.sampleSize}/><span className="sr-only">{suppressionReasonLabel(data.suppressionReason, data.sampleSize)}</span></> : <>
      <div className="histogram" aria-label={mode === "sale" ? "Histogram cene na kvadratni meter" : "Histogram mesečnih najemnin"}><span className="histogram-y-axis">Število poslov</span>{data.buckets.map((bucket) => { const label = `${format.format(bucket.from)}–${format.format(bucket.to)} ${unit}: ${countLabel(bucket.count)}`; return <button key={bucket.index} type="button" className="histogram-bar" style={{ height: `${Math.max(10, bucket.count / max * 100)}%` }} aria-label={label}><span className="histogram-tooltip" role="tooltip"><strong>{countLabel(bucket.count)}</strong><small>{format.format(bucket.from)}–{format.format(bucket.to)} {unit}</small></span></button>; })}<span className="histogram-x-axis">{mode === "sale" ? "Cena na m²" : "Mesečna najemnina"}</span></div>
      <div className="percentile-visual" aria-label="Percentili reprezentativne porazdelitve"><div className="percentile-axis"><i/><i/><i/><i/><i/></div><div className="percentile-values">{(["p10", "p25", "p50", "p75", "p90"] as const).map((key) => <div key={key} className={key === "p50" ? "median" : ""}><span>{key === "p50" ? "Mediana" : key.toUpperCase()}</span><strong>{format.format(data.percentiles![key])} {unit}</strong></div>)}</div><p><strong>P25–P75</strong> predstavlja srednjih 50 % opazovane reprezentativne porazdelitve.</p></div>
    </>}
  </section>;
}
