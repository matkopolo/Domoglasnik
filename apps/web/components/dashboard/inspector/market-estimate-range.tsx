import type { CurrentMarketEstimate, MarketMode } from "@/lib/types";
import { decimal, euro } from "@/lib/display-values";
import { ConfidenceBadge } from "@/components/ui/confidence-badge";
import { ProvenanceBadge } from "@/components/ui/provenance-badge";

export function MarketEstimateRange({ estimate, mode }: { estimate: CurrentMarketEstimate; mode: MarketMode }) {
  const radius = estimate.maximumRadiusMeters >= 1000
    ? `${decimal.format(estimate.maximumRadiusMeters / 1000)} km` : `${estimate.maximumRadiusMeters} m`;
  return <section className="market-estimate" aria-labelledby="market-estimate-title">
    <header><div><ProvenanceBadge kind="calculated" />
      <h3 id="market-estimate-title">{mode === "sale" ? "Ocena trenutne tržne vrednosti" : "Ocena trenutne mesečne najemnine"}</h3>
      <p>Ocena temelji na najnovejših razpoložljivih primerljivih poslih, ne na letu izbranega posla.</p></div>
      <ConfidenceBadge confidence={estimate.confidence} /></header>
    <span className="estimate-range-caption">Primerljivi posli brez lokalne prilagoditve</span>
    <div className="estimate-range-values"><span>{euro.format(estimate.unadjustedLowerTotal)}</span>
      <strong>{euro.format(estimate.unadjustedCentralTotal)}</strong><span>{euro.format(estimate.unadjustedUpperTotal)}</span></div>
    <div className="estimate-range-track" aria-label={`P25 ${euro.format(estimate.unadjustedLowerTotal)}, mediana ${euro.format(estimate.unadjustedCentralTotal)}, P75 ${euro.format(estimate.unadjustedUpperTotal)}`}>
      <i /><i /><i />
    </div>
    <div className="estimate-range-labels"><span>P25</span><strong>Mediana</strong><span>P75</span></div>
    <strong className="estimate-unit-value">{decimal.format(estimate.unadjustedCentralAmountPerSqm)} €/m²{mode === "rent" ? " na mesec" : ""}</strong>
    {estimate.locationAdjustmentApplied && <div className="adjusted-estimate-range">
      <span>Po lokalni prilagoditvi ×{decimal.format(estimate.locationAdjustmentFactor)}</span>
      <strong>{euro.format(estimate.centralTotal)}</strong>
      <small>{euro.format(estimate.lowerTotal)}–{euro.format(estimate.upperTotal)} · {decimal.format(estimate.centralAmountPerSqm)} €/m²</small>
    </div>}
    <p className="estimate-explanation">P25–P75 je srednjih 50 % tehtane porazdelitve primerljivih poslov; ni formalni interval zaupanja ali certificirana cenitev.</p>
    <dl className="estimate-method">
      <div><dt>Primerljivi posli</dt><dd>{estimate.comparableCount}</dd></div>
      <div><dt>Največji radij</dt><dd>{radius}</dd></div>
      <div><dt>Površina</dt><dd>±{estimate.areaTolerancePercent} %</dd></div>
      {estimate.constructionYearTolerance != null && <div><dt>Leto gradnje</dt><dd>±{estimate.constructionYearTolerance} let</dd></div>}
      <div><dt>Referenčno leto</dt><dd>{estimate.referenceYear}</dd></div>
    </dl>
    <p className="estimate-adjustment">Sestava paketa: {estimate.packageCompositionMatched ? "ujemajoče vrste sestavin" : "mešana sestava zaradi premajhnega vzorca"}.</p>
    {estimate.rawLocationAdjustmentFactor !== 1 && <p className="estimate-adjustment">Surovo lokalno razmerje ×{decimal.format(estimate.rawLocationAdjustmentFactor)} · lokalni podporni vzorec n={estimate.localAdjustmentSampleSize}{estimate.locationAdjustmentApplied ? ` · skrčeno na ×${decimal.format(estimate.locationAdjustmentFactor)}` : " · prilagoditev ni uporabljena"}</p>}
    {estimate.warnings.length > 0 && <ul className="estimate-warnings">{estimate.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul>}
  </section>;
}
