"use client";

import { ChevronRight, CircleAlert } from "lucide-react";
import { useState } from "react";
import { ConfidenceBadge } from "@/components/ui/confidence-badge";
import { ProvenanceBadge } from "@/components/ui/provenance-badge";
import { QualityBadge } from "@/components/ui/quality-badge";
import { Tabs } from "@/components/ui/tabs";
import {
  anomalyStatusLabel, decimal, euro, meaningfulPosition, meaningfulText,
  qualityReasonLabel, reliabilityLabel, visibleFields,
} from "@/lib/display-values";
import type {
  ContextBenchmark, FullValuationPrediction, MarketContext, RentalTransactionDetail,
  SaleTransactionDetail, TransactionContextResponse,
} from "@/lib/types";
import { MarketEstimateRange } from "./market-estimate-range";

type Detail = SaleTransactionDetail | RentalTransactionDetail;
type InspectorTab = "overview" | "comparables" | "details" | "quality";

function FieldList({ fields }: { fields: Array<{ label: string; value: string | number | null | undefined }> }) {
  const visible = visibleFields(fields);
  if (!visible.length) return null;
  return <dl className="context-fields">{visible.map((field) => <div key={field.label}><dt>{field.label}</dt><dd>{field.value}</dd></div>)}</dl>;
}

function tierLabel(tier: TransactionContextResponse["directComparables"][number]["matchTier"]) {
  if (tier === "same_building_same_year_area_15") return "Ista stavba · isto leto · površina ±15 %";
  if (tier === "same_building_adjacent_year_area_15") return "Ista stavba · sosednje leto · površina ±15 %";
  if (tier === "same_building_same_year") return "Ista stavba · isto leto · širša površina";
  return "Ista stavba · drug primerljiv posel";
}

function benchmarkProfile(benchmark: ContextBenchmark) {
  if (benchmark.comparisonProfile === "unadjusted") return "širši tržni vzorec";
  const parts = benchmark.areaTolerancePercent ? [`površina ±${benchmark.areaTolerancePercent} %`] : [];
  if (benchmark.constructionYearTolerance) parts.push(`leto gradnje ±${benchmark.constructionYearTolerance} let`);
  if (benchmark.radiusMeters) parts.push(`radij ${benchmark.radiusMeters >= 1000 ? `${decimal.format(benchmark.radiusMeters / 1000)} km` : `${benchmark.radiusMeters} m`}`);
  return parts.join(" · ");
}

export function TransactionInspector({ detail, context, model, onChange }: {
  detail: Detail; context: TransactionContextResponse; model?: FullValuationPrediction;
  onChange: (context: MarketContext) => void;
}) {
  const [tab, setTab] = useState<InspectorTab>("overview");
  const sale = context.transactionKind === "sale" ? detail as SaleTransactionDetail : undefined;
  const rental = context.transactionKind === "rent" ? detail as RentalTransactionDetail : undefined;
  const part = sale?.buildingParts[0];
  const space = rental?.spaces[0];
  const address = [part?.street ?? space?.street, part?.houseNumber ?? space?.houseNumber, part?.settlement ?? space?.settlement].filter(meaningfulText).join(", ");
  const amount = sale?.price ?? rental?.monthlyRentExVat;
  const amountPerSqm = sale?.pricePerSqm ?? rental?.monthlyRentPerSqm;
  const evidence = [
    { label: "Izbrani dejanski posel", detail: "GURS ETN", available: true },
    { label: "Ista stavba", detail: `${context.directComparables.length} neposredno primerljivih poslov`, available: context.directComparables.length > 0 },
    { label: "Fizično primerljiva okolica", detail: "ujemanje površine, starosti in razdalje", available: context.currentMarketEstimate != null },
    { label: "Občina, regija in širši trg", detail: `${context.benchmarks.length} tržnih primerjav`, available: context.benchmarks.length > 0 },
    { label: "Statistični model", detail: "dopolnilna modelska ocena", available: context.valuation != null },
  ].filter((item) => item.available);

  const details = <>
    <FieldList fields={[
      { label: "Datum pogodbe", value: detail.contractDate }, { label: "Naslov", value: address },
      { label: "Velikost", value: detail.area == null ? null : `${decimal.format(detail.area)} m²` },
      { label: "Uporabna površina", value: (part?.usableArea ?? space?.usableArea) == null ? null : `${decimal.format(part?.usableArea ?? space!.usableArea!)} m²` },
      { label: "Leto gradnje", value: part?.constructionYear ?? space?.constructionYear },
      { label: "Nadstropje", value: meaningfulText(part?.floor) }, { label: "Položaj", value: meaningfulPosition(part?.positionInBuilding) },
      { label: "Sobe", value: part?.roomCount }, { label: "Vrsta dela", value: part?.buildingPartTypeDescription },
      { label: "Vrsta prostora", value: space?.rentedSpaceTypeDescription }, { label: "Trajanje najema", value: rental?.rentalTerm },
      { label: "Obratovalni stroški", value: rental?.operatingCostsIncluded == null ? null : rental.operatingCostsIncluded ? "vključeni" : "niso vključeni" },
    ]}/>
    {sale && sale.buildingParts.length + sale.landParcels.length > 1 && <p className="context-note">Posel vsebuje {sale.buildingParts.length} delov stavbe in {sale.landParcels.length} parcel. Cena na m² je izračunana iz celotnega pogodbenega paketa in referenčne površine.</p>}
    {sale?.buildingParts.map((item, index) => <article className="component-detail-card" key={item.id}><strong>{item.buildingPartTypeDescription ?? `Del stavbe ${index + 1}`}</strong><FieldList fields={[
      { label: "Prodana površina", value: item.soldArea == null ? null : `${decimal.format(item.soldArea)} m²` },
      { label: "Prodana uporabna površina", value: item.soldUsableArea == null ? null : `${decimal.format(item.soldUsableArea)} m²` },
      { label: "Registrirana površina", value: item.buildingPartArea == null ? null : `${decimal.format(item.buildingPartArea)} m²` },
      { label: "Nadstropje", value: meaningfulText(item.floor) }, { label: "Položaj", value: meaningfulPosition(item.positionInBuilding) },
      { label: "Pogodbena cena dela", value: item.contractualPrice == null ? null : euro.format(item.contractualPrice) },
    ]}/></article>)}
    {sale?.landParcels.map((parcel) => <article className="component-detail-card" key={parcel.id}><strong>{parcel.landTypeDescription ?? "Zemljišče"}</strong><FieldList fields={[
      { label: "Parcela", value: parcel.parcelNumber }, { label: "Površina", value: parcel.parcelArea == null ? null : `${decimal.format(parcel.parcelArea)} m²` },
      { label: "Pogodbena cena", value: parcel.contractualPrice == null ? null : euro.format(parcel.contractualPrice) },
    ]}/></article>)}
    {rental?.spaces.map((item, index) => <article className="component-detail-card" key={item.id}><strong>{item.rentedSpaceTypeDescription ?? `Oddani prostor ${index + 1}`}</strong><FieldList fields={[
      { label: "Površina", value: item.area == null ? null : `${decimal.format(item.area)} m²` },
      { label: "Uporabna površina", value: item.usableArea == null ? null : `${decimal.format(item.usableArea)} m²` },
      { label: "Pogodbena najemnina", value: item.contractualRent == null ? null : euro.format(item.contractualRent) },
      { label: "Leto gradnje", value: item.constructionYear },
    ]}/></article>)}
  </>;

  return <div className="transaction-inspector">
    <nav className="context-breadcrumbs" aria-label="Lokacija"><button type="button" onClick={() => onChange({ kind: "country" })}>Slovenija</button>
      {detail.geographies.map((geography) => <span key={geography.id}><ChevronRight aria-hidden="true" /><button type="button" onClick={() => onChange({ kind: "geography", ...geography })}>{geography.name}</button></span>)}</nav>
    <Tabs label="transaction-inspector" active={tab} onChange={setTab} tabs={[
      { id: "overview", label: "Pregled" }, { id: "comparables", label: "Primerljivi posli" },
      { id: "details", label: "Podrobnosti" }, { id: "quality", label: "Kakovost in model" },
    ]}>
      {tab === "overview" && <div className="inspector-tab-content">
        <section className="transaction-identity"><ProvenanceBadge kind="source" /><span>{sale ? "Prodaja" : "Najem"} #{detail.transactionId}</span>
          <div className="transaction-amount"><strong>{amount == null ? "Cena ni objavljena" : euro.format(amount)}</strong>{amountPerSqm != null && <b>{decimal.format(amountPerSqm)} €/m²</b>}</div>
          <p>{detail.contractDate ?? "Datum ni objavljen"}{address ? ` · ${address}` : ""}</p>
          <QualityBadge representative={context.quality.isRepresentative}/></section>
        {context.currentMarketEstimate && <><div className="historical-current-divider">
          <div><span>{sale ? "Zabeležena prodajna cena" : "Zabeležena mesečna najemnina"}</span><strong>{detail.contractDate?.slice(0, 4) ?? context.contractYear}</strong></div>
          <i aria-hidden="true"/>
          <div><span>Ocena iz najnovejših primerljivih poslov</span><strong>{context.currentMarketEstimate.referenceYear}</strong></div>
        </div><MarketEstimateRange estimate={context.currentMarketEstimate} mode={context.transactionKind}/></>}
        <section className="evidence-ladder"><h3>Dokazi za primerjavo</h3><ol>{evidence.map((item) => <li key={item.label}><span>{item.label}</span><small>{item.detail}</small></li>)}</ol></section>
      </div>}
      {tab === "comparables" && <div className="inspector-tab-content"><section className="context-section"><span className="section-eyebrow">Neposredni dokazi</span><h3>Posli v isti stavbi</h3>
        <p>Prednost imajo isto leto in najbolj podobna površina.</p>
        {context.directComparables.length === 0 ? <p className="empty-state">V isti stavbi v izbranem ali sosednjih letih ni primerljivega posla.</p> : <div className="comparable-list">{context.directComparables.map((item) => <button className="direct-comparable-card" type="button" key={item.transactionId} onClick={() => onChange({ kind: "transaction", mode: context.transactionKind, id: item.transactionId })}>
          <span className="comparable-tier">{tierLabel(item.matchTier)}</span><div><span>{item.contractDate}<small>Posel #{item.transactionId}</small></span><strong>{euro.format(item.amount)}<small>{decimal.format(item.amountPerSqm)} €/m²</small></strong></div>
          <dl><div><dt>Površina</dt><dd>{decimal.format(item.area)} m²</dd></div><div><dt>Razlika površine</dt><dd>{item.areaDifferencePercent > 0 ? "+" : ""}{decimal.format(item.areaDifferencePercent)} %</dd></div><div><dt>Razlika €/m²</dt><dd>{item.amountPerSqmDifferencePercent > 0 ? "+" : ""}{decimal.format(item.amountPerSqmDifferencePercent)} %</dd></div></dl>
          <QualityBadge representative={item.isRepresentative} label={item.isRepresentative ? "Reprezentativen posel" : anomalyStatusLabel(item.outlierStatus)}/></button>)}</div>}
      </section><section className="context-section"><span className="section-eyebrow">Širši tržni kontekst</span><h3>Primerjava z okolico</h3>
        {context.incompletePeriod && <p className="context-warning"><CircleAlert aria-hidden="true"/> Leto še ni zaključeno.</p>}
        {context.benchmarks.length === 0 ? <p className="empty-state">Za enako vrsto nepremičnine in isto leto ni dovolj primerljivih poslov.</p> : context.benchmarks.map((item) => <article className="benchmark-card" key={`${item.scope}-${item.geographyId ?? item.radiusMeters ?? 0}`}><div><strong>{item.label}</strong><span>{item.period} · n={item.sampleSize}</span></div><b className="direction-neutral">{item.differencePercent > 0 ? "+" : ""}{decimal.format(item.differencePercent)} % {item.differencePercent >= 0 ? "nad" : "pod"} mediano</b><small>mediana {decimal.format(item.medianAmountPerSqm)} €/m² · zanesljivost {reliabilityLabel(item.reliability)}</small><small>izbor: {benchmarkProfile(item)}</small></article>)}
      </section></div>}
      {tab === "details" && <div className="inspector-tab-content"><section className="context-section"><ProvenanceBadge kind="source"/><h3>Podrobnosti pogodbenega posla</h3>{details}</section></div>}
      {tab === "quality" && <div className="inspector-tab-content"><section className="context-section quality-evidence"><h3>Kakovost podatka</h3><QualityBadge representative={context.quality.isRepresentative}/><FieldList fields={[{ label: "Kakovostno ustrezen", value: context.quality.isQualityEligible ? "Da" : "Ne" }, { label: "Status odstopanja", value: anomalyStatusLabel(context.outlierStatus) }]}/>{context.quality.reasons.length > 0 && <ul>{context.quality.reasons.map((reason) => <li key={reason}>{qualityReasonLabel(reason)}</li>)}</ul>}<strong className="quality-principle">Statistično odstopanje ni nujno napaka.</strong></section>
        {context.valuation && <section className="context-section model-evidence"><ProvenanceBadge kind="model"/><h3>Statistična ocena</h3><strong>{decimal.format(context.valuation.predictedAmountPerSqm)} €/m²</strong><p>Razpon {decimal.format(context.valuation.lowerAmountPerSqm)}–{decimal.format(context.valuation.upperAmountPerSqm)} €/m²</p><ConfidenceBadge confidence={context.valuation.reliability}/>{context.valuation.warnings.length > 0 && <ul>{context.valuation.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul>}<small>Statistična ocena ni uradna cenitev.</small></section>}
        {model && <details className="model-diagnostics"><summary>Prikaži diagnostiko modela</summary><ProvenanceBadge kind="model"/><FieldList fields={[
          { label: "Model", value: model.modelType }, { label: "Različica", value: model.modelVersion }, { label: "Učni vzorec", value: model.trainingRows.toLocaleString("sl-SI") },
          { label: "Log ostanek", value: decimal.format(model.logResidual) },
          { label: "Napaka MAPE", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal?.median_absolute_percentage_error == null ? null : `${decimal.format(100 * model.metrics.property_groups[model.propertyGroup]!.temporal!.median_absolute_percentage_error!)} %` },
          { label: "Pokritost intervala", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal?.empirical_interval_coverage == null ? null : `${decimal.format(100 * model.metrics.property_groups[model.propertyGroup]!.temporal!.empirical_interval_coverage!)} %` },
          { label: "Časovni R²", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal?.r2_log == null ? null : decimal.format(model.metrics.property_groups[model.propertyGroup]!.temporal!.r2_log!) },
          { label: "Prostorski R²", value: model.metrics.property_groups?.[model.propertyGroup]?.spatial?.r2_log == null ? null : decimal.format(model.metrics.property_groups[model.propertyGroup]!.spatial!.r2_log!) },
        ]}/><p>Diagnostika opisuje napako modela, ne gotovosti posamezne cenitve.</p></details>}
      </div>}
    </Tabs>
  </div>;
}
