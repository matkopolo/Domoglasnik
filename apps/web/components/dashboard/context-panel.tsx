"use client";

import { ArrowRight, ChevronRight, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  loadBuildingStatistics, loadFullValuation, loadGeography, loadGeographyChildren,
  loadGeographyComparisons, loadMarketIndices, loadTransaction, loadTransactionContext,
} from "@/lib/api";
import { decimal, euro, indexChangeInterpretation, propertyGroupLabel, visibleFields } from "@/lib/display-values";
import type {
  BuildingStatistics, FullValuationPrediction, Geography, GeographyComparison, MarketContext,
  MarketIndex, MarketMode, Overview, PropertyGroup, RentalTransactionDetail,
  SaleTransactionDetail, TransactionContextResponse, YearRange,
} from "@/lib/types";
import { ProvenanceBadge } from "@/components/ui/provenance-badge";
import { SuppressedStatistic } from "@/components/ui/suppressed-statistic";
import { TransactionInspector } from "./inspector/transaction-inspector";

type LoadedContext =
  | { kind: "geography"; geography: Geography; children: Geography[]; comparisons: GeographyComparison[] }
  | { kind: "building"; building: BuildingStatistics }
  | { kind: "transaction"; detail: SaleTransactionDetail | RentalTransactionDetail; context: TransactionContextResponse; model?: FullValuationPrediction };

interface ContextPanelProps {
  context: MarketContext; mode: MarketMode; propertyGroup: PropertyGroup; years: YearRange;
  overview: Overview; onChange: (context: MarketContext) => void;
}

function contextKey(context: MarketContext) {
  if (context.kind === "country") return "country";
  if (context.kind === "geography") return `geography:${context.id}`;
  if (context.kind === "building") return `building:${context.cadastralCode}:${context.buildingNumber}`;
  return `transaction:${context.mode}:${context.id}`;
}

function FieldList({ fields }: { fields: Array<{ label: string; value: string | number | null | undefined }> }) {
  const visible = visibleFields(fields);
  if (!visible.length) return null;
  return <dl className="context-fields">{visible.map((field) => <div key={field.label}><dt>{field.label}</dt><dd>{field.value}</dd></div>)}</dl>;
}

function GeographyInspector({ loaded, mode, propertyGroup, years, onChange }: {
  loaded: Extract<LoadedContext, { kind: "geography" }>; mode: MarketMode; propertyGroup: PropertyGroup;
  years: YearRange; onChange: (context: MarketContext) => void;
}) {
  const [showAllChildren, setShowAllChildren] = useState(false);
  const [showAllComparisons, setShowAllComparisons] = useState(false);
  const visibleChildren = showAllChildren ? loaded.children : loaded.children.slice(0, 10);
  const visibleComparisons = showAllComparisons ? loaded.comparisons : loaded.comparisons.slice(0, 10);
  return <div className="geography-inspector"><div className="context-breadcrumbs"><button type="button" onClick={() => onChange({ kind: "country" })}>Slovenija</button><ChevronRight aria-hidden="true"/><span>{loaded.geography.name}</span></div>
    <p>Prikazane statistike, grafi in zemljevid uporabljajo isti filter: {propertyGroupLabel(propertyGroup)}, {years.from}–{years.to}.</p>
    <section className="context-section"><h3>Podrejena območja <small>({loaded.children.length})</small></h3>{loaded.children.length === 0 ? <p>Ni podrejenih območij.</p> : <><div className="context-children">{visibleChildren.map((child) => <button type="button" key={child.id} onClick={() => onChange({ kind: "geography", ...child })}>{child.name}</button>)}</div>{loaded.children.length > 10 && <button type="button" className="context-list-toggle" aria-expanded={showAllChildren} onClick={() => setShowAllChildren((value) => !value)}>{showAllChildren ? "Prikaži manj" : `Prikaži vseh ${loaded.children.length} območij`}</button>}</>}</section>
    {loaded.comparisons.length > 0 && <section className="context-section"><h3>Primerjava podrejenih območij</h3>{visibleComparisons.map((item) => <button className="context-transaction" type="button" key={item.id} onClick={() => onChange({ kind: "geography", id: item.id, level: item.level, name: item.name })}><span>{item.name}<small>{item.transactionCount.toLocaleString("sl-SI")} vseh · {item.representativeCount.toLocaleString("sl-SI")} reprezentativnih</small></span><strong>{(mode === "rent" ? item.medianAmount : item.medianAmountPerSqm) == null ? "Mediana ni objavljena" : mode === "rent" ? `${euro.format(item.medianAmount!)} / mesec` : `${decimal.format(item.medianAmountPerSqm!)} €/m²`}</strong></button>)}{loaded.comparisons.length > 10 && <button type="button" className="context-list-toggle" aria-expanded={showAllComparisons} onClick={() => setShowAllComparisons((value) => !value)}>{showAllComparisons ? "Prikaži manj" : `Prikaži vseh ${loaded.comparisons.length} primerjav`}</button>}</section>}
  </div>;
}

function BuildingInspector({ building, mode, onChange }: { building: BuildingStatistics; mode: MarketMode; onChange: (context: MarketContext) => void }) {
  const summary = mode === "sale" ? building.sales : building.rentals;
  const history = mode === "sale" ? building.saleHistory : building.rentalHistory;
  const transactions = mode === "sale" ? building.saleTransactions : building.rentalTransactions;
  const amount = (point: typeof history[number]) => mode === "sale" ? point.medianAmountPerSqm : point.medianAmount;
  const max = Math.max(...history.map((point) => amount(point) ?? 0), 1);
  return <div className="building-inspector"><section className="building-identity"><span className="section-eyebrow">Stavba</span><h3>Katastrska stavba {building.buildingNumber}</h3><FieldList fields={[
    { label: "Katastrska občina", value: building.cadastralMunicipalityCode }, { label: "Leto gradnje", value: building.constructionYear }, { label: "Znani deli", value: building.knownBuildingPartCount },
  ]}/></section>
    <div className="building-market-kpis"><div><span>Vsi posli</span><strong>{summary.transactionCount.toLocaleString("sl-SI")}</strong></div><div><span>Reprezentativni</span><strong>{summary.representativeCount.toLocaleString("sl-SI")}</strong></div><div><span>{mode === "sale" ? "Mediana / m²" : "Mesečna mediana"}</span><strong>{(mode === "sale" ? summary.medianAmountPerSqm : summary.medianAmount) == null ? "—" : mode === "sale" ? `${decimal.format(summary.medianAmountPerSqm!)} €/m²` : `${euro.format(summary.medianAmount!)} / mesec`}</strong></div></div>
    {history.length > 0 && <section className="context-section"><h3>Zgodovina stavbe</h3><div className="building-history">{history.map((point) => <div key={point.year}><span>{point.year}</span><i style={{ height: `${Math.max(8, 100 * (amount(point) ?? 0) / max)}%` }}/><strong>{amount(point) == null ? "—" : mode === "sale" ? decimal.format(amount(point)!) : euro.format(amount(point)!)}</strong><small>n={point.representativeCount}</small></div>)}</div></section>}
    <FieldList fields={[{ label: "Prvi posel", value: summary.firstContractDate }, { label: "Zadnji posel", value: summary.lastContractDate }]}/>
    <section className="context-section building-transactions"><h3>{mode === "sale" ? "Prodaje" : "Najemi"} v stavbi</h3>{transactions.length === 0 ? <p className="empty-state">V izbranem obdobju ni poslov.</p> : <><p className="building-transactions-hint">Izberite posel za celotne podatke, primerljive posle in tržno oceno.</p><div className="building-transaction-list">{transactions.map((item) => <button className="building-transaction" type="button" key={item.transactionId} aria-label={`Odpri analizo ${mode === "sale" ? "prodaje" : "najema"} #${item.transactionId}`} onClick={() => onChange({ kind: "transaction", mode, id: item.transactionId })}>
      <span className="building-transaction-heading"><strong>{mode === "sale" ? "Prodaja" : "Najem"} #{item.transactionId}</strong><span>{item.contractDate ?? "Datum ni objavljen"}</span></span>
      <span className="building-transaction-evidence"><strong>{item.amount == null ? "Cena ni objavljena" : euro.format(item.amount)}</strong><span>{item.amountPerSqm == null ? "Cena / m² ni objavljena" : `${decimal.format(item.amountPerSqm)} €/m²`}{item.area == null ? "" : ` · ${decimal.format(item.area)} m²`}</span></span>
      <span className="building-transaction-action">Odpri analizo posla <ArrowRight aria-hidden="true"/></span>
    </button>)}</div></>}</section>
  </div>;
}

export function ContextPanel({ context, mode, propertyGroup, years, overview, onChange }: ContextPanelProps) {
  const [loaded, setLoaded] = useState<LoadedContext>();
  const [loadedKey, setLoadedKey] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [indices, setIndices] = useState<MarketIndex[]>([]);
  const generation = useRef(0);
  const key = contextKey(context);

  useEffect(() => {
    if (context.kind === "country") return;
    const controller = new AbortController();
    const currentGeneration = ++generation.current;
    queueMicrotask(() => { if (!controller.signal.aborted) { setLoading(true); setError(undefined); } });
    const request: Promise<LoadedContext> = context.kind === "geography"
      ? Promise.all([loadGeography(context.id, controller.signal), loadGeographyChildren(context.id, controller.signal)]).then(async ([geography, children]) => ({ kind: "geography", geography, children: children.items, comparisons: await loadGeographyComparisons(mode, geography, children.items, propertyGroup, years, controller.signal) }))
      : context.kind === "building"
        ? loadBuildingStatistics(context.cadastralCode, context.buildingNumber, propertyGroup, years, controller.signal).then((building) => ({ kind: "building", building }))
        : Promise.all([loadTransaction(context.mode, context.id, controller.signal), loadTransactionContext(context.mode, context.id, controller.signal), loadFullValuation(context.mode, context.id, controller.signal).catch(() => undefined)]).then(([detail, transactionContext, model]) => ({ kind: "transaction", detail, context: transactionContext, model }));
    request.then((result) => { if (generation.current === currentGeneration) { setLoaded(result); setLoadedKey(key); } }).catch((requestError: unknown) => {
      if (!controller.signal.aborted && generation.current === currentGeneration) setError(requestError instanceof Error ? requestError.message : "Podatkov ni bilo mogoče naložiti.");
    }).finally(() => { if (!controller.signal.aborted && generation.current === currentGeneration) setLoading(false); });
    return () => controller.abort();
  }, [context, key, mode, propertyGroup, years]);

  useEffect(() => {
    if (context.kind !== "country") return;
    const controller = new AbortController();
    loadMarketIndices(mode, propertyGroup, controller.signal).then((response) => setIndices(response.items)).catch(() => { if (!controller.signal.aborted) setIndices([]); });
    return () => controller.abort();
  }, [context.kind, mode, propertyGroup]);

  const current = loadedKey === key ? loaded : undefined;
  const latestIndices = (["hedonic", "matched_pair_median"] as const).map((method) => indices.filter((item) => item.method === method && item.isPublishable).at(-1)).filter((item): item is MarketIndex => item != null);
  const title = context.kind === "country" ? "Slovenija" : context.kind === "geography" ? context.name : context.kind === "building" ? context.label ?? `Stavba ${context.buildingNumber}` : `${context.mode === "sale" ? "Prodaja" : "Najem"} #${context.id}`;
  return <aside className="context-panel selection-inspector" aria-live="polite" aria-busy={loading}>
    <header><div><span>Izbrani kontekst</span><h2>{title}</h2></div>{context.kind !== "country" && <button type="button" className="icon-button" aria-label="Zapri podrobnosti" onClick={() => onChange({ kind: "country" })}><X/></button>}</header>
    {loading && <div className="context-progress" role="status">Osvežujem izbrani kontekst …</div>}
    {error && <p className="context-error" role="alert">{error} Prejšnji uspešni podatki so ostali prikazani.</p>}
    {context.kind === "country" && <div className="country-inspector"><ProvenanceBadge kind="calculated"/><div className="context-primary"><span>{mode === "sale" ? "Mediana cene / m²" : "Mediana mesečne najemnine"}</span><strong>{(mode === "sale" ? overview.medianPricePerSqm : overview.medianMonthlyRent) == null ? "Ni dovolj podatkov" : mode === "sale" ? `${decimal.format(overview.medianPricePerSqm!)} €/m²` : `${euro.format(overview.medianMonthlyRent!)} / mesec`}</strong></div><p>Izberite območje, stavbo ali posel za natančnejše dokaze.</p>
      {latestIndices.length > 0 && <section className="context-section"><h3>Nacionalni indeks cen</h3>{latestIndices.map((index) => <article className="benchmark-card" key={index.method}><div><strong>{index.method === "hedonic" ? "Hedonski indeks" : "Indeks ponovnih prodaj"}</strong><span>začetno leto {index.basePeriod.slice(0, 4)} · n={index.sampleSize}</span></div><b>{indexChangeInterpretation(index.indexValue, index.basePeriod)}</b><small>indeks {decimal.format(index.indexValue)} · {index.period.slice(0, 7)}</small></article>)}<small>Nad 100 pomeni višje, pod 100 nižje cene glede na začetno leto, ko je indeks nastavljen na 100.</small></section>}
    </div>}
    {context.kind !== "country" && !loading && !error && !current && <SuppressedStatistic/>}
    {current?.kind === "geography" && <GeographyInspector key={current.geography.id} loaded={current} mode={mode} propertyGroup={propertyGroup} years={years} onChange={onChange}/>}
    {current?.kind === "building" && <BuildingInspector building={current.building} mode={mode} onChange={onChange}/>}
    {current?.kind === "transaction" && <TransactionInspector detail={current.detail} context={current.context} model={current.model} onChange={onChange}/>}
  </aside>;
}
