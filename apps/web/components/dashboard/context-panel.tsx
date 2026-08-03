"use client";

import { ChevronRight, CircleAlert, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  loadBuildingStatistics,
  loadGeography,
  loadGeographyChildren,
  loadGeographyComparisons,
  loadFullValuation,
  loadMarketIndices,
  loadTransaction,
  loadTransactionContext,
} from "@/lib/api";
import {
  anomalyStatusLabel,
  decimal,
  euro,
  meaningfulPosition,
  meaningfulText,
  propertyGroupLabel,
  qualityReasonLabel,
  reliabilityLabel,
  visibleFields,
} from "@/lib/display-values";
import type {
  BuildingStatistics,
  ContextBenchmark,
  Geography,
  GeographyComparison,
  FullValuationPrediction,
  MarketContext,
  MarketIndex,
  MarketMode,
  Overview,
  PropertyGroup,
  RentalTransactionDetail,
  SaleTransactionDetail,
  TransactionContextResponse,
  YearRange,
} from "@/lib/types";

type LoadedContext =
  | {
      kind: "geography";
      geography: Geography;
      children: Geography[];
      comparisons: GeographyComparison[];
    }
  | { kind: "building"; building: BuildingStatistics }
  | {
      kind: "transaction";
      detail: SaleTransactionDetail | RentalTransactionDetail;
      context: TransactionContextResponse;
      model?: FullValuationPrediction;
    };

interface ContextPanelProps {
  context: MarketContext;
  mode: MarketMode;
  propertyGroup: PropertyGroup;
  years: YearRange;
  overview: Overview;
  onChange: (context: MarketContext) => void;
}

function contextKey(context: MarketContext) {
  if (context.kind === "country") return "country";
  if (context.kind === "geography") return `geography:${context.id}`;
  if (context.kind === "building") {
    return `building:${context.cadastralCode}:${context.buildingNumber}`;
  }
  return `transaction:${context.mode}:${context.id}`;
}

function comparableTierLabel(tier: TransactionContextResponse["directComparables"][number]["matchTier"]) {
  switch (tier) {
    case "same_building_same_year_area_15": return "Ista stavba · isto leto · površina ±15 %";
    case "same_building_adjacent_year_area_15": return "Ista stavba · sosednje leto · površina ±15 %";
    case "same_building_same_year": return "Ista stavba · isto leto · širša razlika v površini";
    default: return "Ista stavba · drug primerljiv posel";
  }
}

function benchmarkProfileLabel(benchmark: ContextBenchmark) {
  if (benchmark.comparisonProfile === "unadjusted") return "širši tržni vzorec";
  const parts = benchmark.areaTolerancePercent
    ? [`površina ±${benchmark.areaTolerancePercent} %`]
    : [];
  if (benchmark.constructionYearTolerance) {
    parts.push(`leto gradnje ±${benchmark.constructionYearTolerance} let`);
  }
  return parts.join(" · ");
}

function FieldList({ fields }: {
  fields: Array<{ label: string; value: string | number | null | undefined }>;
}) {
  const visible = visibleFields(fields);
  if (visible.length === 0) return null;
  return (
    <dl className="context-fields">
      {visible.map((field) => (
        <div key={field.label}>
          <dt>{field.label}</dt>
          <dd>{field.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function TransactionView({
  loaded,
  onChange,
}: {
  loaded: Extract<LoadedContext, { kind: "transaction" }>;
  onChange: (context: MarketContext) => void;
}) {
  const { detail, context, model } = loaded;
  const sale = context.transactionKind === "sale"
    ? detail as SaleTransactionDetail
    : undefined;
  const rental = context.transactionKind === "rent"
    ? detail as RentalTransactionDetail
    : undefined;
  const part = sale?.buildingParts[0];
  const space = rental?.spaces[0];
  const position = meaningfulPosition(part?.positionInBuilding);
  const address = [
    part?.street ?? space?.street,
    part?.houseNumber ?? space?.houseNumber,
    part?.settlement ?? space?.settlement,
  ].filter(meaningfulText).join(", ");
  const amount = sale?.price ?? rental?.monthlyRentExVat;
  const amountPerSqm = sale?.pricePerSqm ?? rental?.monthlyRentPerSqm;

  return (
    <>
      <div className="context-breadcrumbs" aria-label="Lokacija">
        <button type="button" onClick={() => onChange({ kind: "country" })}>Slovenija</button>
        {detail.geographies.map((geography) => (
          <span key={geography.id}>
            <ChevronRight size={13} />
            <button type="button" onClick={() => onChange({
              kind: "geography",
              id: geography.id,
              level: geography.level,
              name: geography.name,
            })}>{geography.name}</button>
          </span>
        ))}
      </div>
      <div className="context-primary">
        <small className="provenance-label source">Podatek GURS ETN</small>
        <span>{sale ? "Prodaja" : "Najem"} #{detail.transactionId}</span>
        <strong>{amount == null ? "Cena ni objavljena" : euro.format(amount)}</strong>
        {amountPerSqm != null && <b>{decimal.format(amountPerSqm)} €/m²</b>}
      </div>
      <FieldList fields={[
        { label: "Datum pogodbe", value: detail.contractDate },
        { label: "Naslov", value: address },
        { label: "Velikost", value: detail.area == null ? null : `${decimal.format(detail.area)} m²` },
        { label: "Leto gradnje", value: part?.constructionYear ?? space?.constructionYear },
        { label: "Nadstropje", value: meaningfulText(part?.floor) },
        { label: "Položaj", value: position },
        { label: "Sobe", value: part?.roomCount },
        { label: "Uporabna površina", value: (part?.usableArea ?? space?.usableArea) == null
          ? null : `${decimal.format(part?.usableArea ?? space!.usableArea!)} m²` },
        { label: "Vrsta dela", value: part?.buildingPartTypeDescription },
        { label: "Vrsta prostora", value: space?.rentedSpaceTypeDescription },
        { label: "Trajanje najema", value: rental?.rentalTerm },
        { label: "Obratovalni stroški", value: rental?.operatingCostsIncluded == null
          ? null : rental.operatingCostsIncluded ? "vključeni" : "niso vključeni" },
      ]} />
      {sale && sale.buildingParts.length + sale.landParcels.length > 1 && (
        <p className="context-note">
          Posel vsebuje {sale.buildingParts.length} delov stavbe in {sale.landParcels.length} parcel.
          Cena na m² je izračunana iz celotnega pogodbenega paketa in referenčne površine.
        </p>
      )}
      {sale?.buildingParts.map((buildingPart, index) => (
        <article className="component-detail-card" key={buildingPart.id}>
          <strong>{buildingPart.buildingPartTypeDescription ?? `Del stavbe ${index + 1}`}</strong>
          <FieldList fields={[
            { label: "Prodana površina", value: buildingPart.soldArea == null
              ? null : `${decimal.format(buildingPart.soldArea)} m²` },
            { label: "Prodana uporabna površina", value: buildingPart.soldUsableArea == null
              ? null : `${decimal.format(buildingPart.soldUsableArea)} m²` },
            { label: "Registrirana površina", value: buildingPart.buildingPartArea == null
              ? null : `${decimal.format(buildingPart.buildingPartArea)} m²` },
            { label: "Nadstropje", value: meaningfulText(buildingPart.floor) },
            { label: "Položaj", value: meaningfulPosition(buildingPart.positionInBuilding) },
            { label: "Pogodbena cena dela", value: buildingPart.contractualPrice == null
              ? null : euro.format(buildingPart.contractualPrice) },
          ]} />
        </article>
      ))}
      {sale?.landParcels.map((parcel) => (
        <div className="component-card" key={parcel.id}>
          <strong>{parcel.landTypeDescription ?? "Zemljišče"}</strong>
          {parcel.parcelArea != null && <span>{decimal.format(parcel.parcelArea)} m²</span>}
          {parcel.contractualPrice != null && <span>{euro.format(parcel.contractualPrice)}</span>}
        </div>
      ))}
      {rental?.spaces.map((rentalSpace, index) => (
        <article className="component-detail-card" key={rentalSpace.id}>
          <strong>{rentalSpace.rentedSpaceTypeDescription ?? `Oddani prostor ${index + 1}`}</strong>
          <FieldList fields={[
            { label: "Površina", value: rentalSpace.area == null
              ? null : `${decimal.format(rentalSpace.area)} m²` },
            { label: "Uporabna površina", value: rentalSpace.usableArea == null
              ? null : `${decimal.format(rentalSpace.usableArea)} m²` },
            { label: "Pogodbena najemnina", value: rentalSpace.contractualRent == null
              ? null : euro.format(rentalSpace.contractualRent) },
            { label: "Leto gradnje", value: rentalSpace.constructionYear },
          ]} />
        </article>
      ))}
      <section className="context-section">
        <h3>Neposredno primerljivi posli</h3>
        <p className="context-note">
          Prednost imajo posli v isti katastrski stavbi, nato isto leto in najbolj podobna površina.
        </p>
        {context.directComparables.length === 0 ? (
          <p>V isti stavbi v izbranem ali sosednjih letih ni primerljivega posla.</p>
        ) : context.directComparables.map((comparable) => (
          <button
            className="direct-comparable-card"
            type="button"
            key={comparable.transactionId}
            onClick={() => onChange({
              kind: "transaction",
              mode: context.transactionKind,
              id: comparable.transactionId,
            })}
          >
            <span>{comparableTierLabel(comparable.matchTier)}</span>
            <strong>{euro.format(comparable.amount)}</strong>
            <b>{decimal.format(comparable.amountPerSqm)} €/m²</b>
            <small>
              {comparable.contractDate} · {decimal.format(comparable.area)} m² · površina
              {comparable.areaDifferencePercent >= 0 ? " +" : " "}
              {decimal.format(comparable.areaDifferencePercent)} %
            </small>
            <small className={comparable.isRepresentative ? "positive" : "negative"}>
              {comparable.isRepresentative
                ? "reprezentativen posel"
                : `preveri kakovost${comparable.outlierStatus ? ` · ${comparable.outlierStatus}` : ""}`}
            </small>
          </button>
        ))}
      </section>
      {context.currentMarketEstimate && (
        <section className="context-section current-market-estimate">
          <div className="context-section-heading">
            <h3>{context.transactionKind === "sale"
              ? "Ocena trenutne tržne vrednosti"
              : "Ocena trenutne mesečne najemnine"}</h3>
            <small className="provenance-label calculated">Izračun aplikacije</small>
          </div>
          <div className="current-market-range">
            <span>{euro.format(context.currentMarketEstimate.lowerTotal)}</span>
            <strong>{euro.format(context.currentMarketEstimate.centralTotal)}</strong>
            <span>{euro.format(context.currentMarketEstimate.upperTotal)}</span>
          </div>
          <p>
            Osrednja ocena {decimal.format(context.currentMarketEstimate.centralAmountPerSqm)} €/m²
            {context.transactionKind === "rent" ? " na mesec" : ""}
            · zanesljivost {reliabilityLabel(context.currentMarketEstimate.confidence)}
          </p>
          <div className="current-market-kpis">
            <small><b>{context.currentMarketEstimate.comparableCount}</b> primerljivih poslov</small>
            <small><b>{context.currentMarketEstimate.maximumRadiusMeters >= 1000
              ? `${decimal.format(context.currentMarketEstimate.maximumRadiusMeters / 1000)} km`
              : `${context.currentMarketEstimate.maximumRadiusMeters} m`}</b> največji radij</small>
            <small><b>±{context.currentMarketEstimate.areaTolerancePercent} %</b> površina</small>
            {context.currentMarketEstimate.constructionYearTolerance && (
              <small><b>±{context.currentMarketEstimate.constructionYearTolerance} let</b> leto gradnje</small>
            )}
          </div>
          {context.currentMarketEstimate.locationAdjustmentFactor !== 1 && (
            <small>
              Lokalna prilagoditev ×{decimal.format(context.currentMarketEstimate.locationAdjustmentFactor)}
              · lokalni zgodovinski vzorec n={context.currentMarketEstimate.localAdjustmentSampleSize}
            </small>
          )}
          <ul>{context.currentMarketEstimate.warnings.map((warning) => (
            <li key={warning}>{warning}</li>
          ))}</ul>
        </section>
      )}
      <section className="context-section">
        <h3>Primerjava z okolico</h3>
        {context.incompletePeriod && (
          <p className="context-warning"><CircleAlert size={14} /> Leto še ni zaključeno.</p>
        )}
        {context.benchmarks.length === 0 ? (
          <p>Za enako vrsto nepremičnine in isto leto ni dovolj primerljivih poslov.</p>
        ) : context.benchmarks.map((benchmark) => (
          <article className="benchmark-card" key={`${benchmark.scope}-${benchmark.geographyId ?? benchmark.radiusMeters ?? 0}`}>
            <div><strong>{benchmark.label}</strong><span>{benchmark.period} · n={benchmark.sampleSize}</span></div>
            <b className={benchmark.differencePercent > 0 ? "negative" : "positive"}>
              {benchmark.differencePercent > 0 ? "+" : ""}{decimal.format(benchmark.differencePercent)} %
            </b>
            <small>
              mediana {decimal.format(benchmark.medianAmountPerSqm)} €/m² ·
              zanesljivost {reliabilityLabel(benchmark.reliability)}
            </small>
            <small>izbor: {benchmarkProfileLabel(benchmark)}</small>
          </article>
        ))}
      </section>
      <section className="context-section">
        <h3>Kakovost podatka</h3>
        <div className="quality-status">
          <strong>{context.quality.isRepresentative
            ? "Reprezentativen posel"
            : "Posel ni v reprezentativnem vzorcu"}</strong>
          {context.outlierStatus && <span>{anomalyStatusLabel(context.outlierStatus)}</span>}
        </div>
        {context.quality.reasons.length > 0 && (
          <ul>{context.quality.reasons.map((reason) => (
            <li key={reason}>{qualityReasonLabel(reason)}</li>
          ))}</ul>
        )}
      </section>
      {context.valuation && (
        <section className="context-section">
          <small className="provenance-label calculated">Statistična modelska ocena</small>
          <h3>Statistična ocena</h3>
          <strong>{decimal.format(context.valuation.predictedAmountPerSqm)} €/m²</strong>
          <p>
            Razpon {decimal.format(context.valuation.lowerAmountPerSqm)}–
            {decimal.format(context.valuation.upperAmountPerSqm)} €/m² ·
            zanesljivost {reliabilityLabel(context.valuation.reliability)}
          </p>
          <small>Statistična ocena ni uradna cenitev.</small>
        </section>
      )}
      {model && (
        <section className="context-section model-diagnostics">
          <h3>Diagnostika modela</h3>
          <FieldList fields={[
            { label: "Model", value: model.modelType },
            { label: "Različica", value: model.modelVersion },
            { label: "Učni vzorec", value: model.trainingRows.toLocaleString("sl-SI") },
            { label: "Log ostanek", value: decimal.format(model.logResidual) },
            { label: "Napaka MAPE", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal
              ?.median_absolute_percentage_error == null ? null
              : `${decimal.format(100 * model.metrics.property_groups[model.propertyGroup]!.temporal!.median_absolute_percentage_error!)} %` },
            { label: "Pokritost intervala", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal
              ?.empirical_interval_coverage == null ? null
              : `${decimal.format(100 * model.metrics.property_groups[model.propertyGroup]!.temporal!.empirical_interval_coverage!)} %` },
            { label: "Časovni R²", value: model.metrics.property_groups?.[model.propertyGroup]?.temporal?.r2_log == null
              ? null : decimal.format(model.metrics.property_groups[model.propertyGroup]!.temporal!.r2_log!) },
            { label: "Prostorski R²", value: model.metrics.property_groups?.[model.propertyGroup]?.spatial?.r2_log == null
              ? null : decimal.format(model.metrics.property_groups[model.propertyGroup]!.spatial!.r2_log!) },
          ]} />
          <p>Ostanek kaže odmik posla od modela. Diagnostika opisuje napako modela, ne gotovosti posamezne cenitve.</p>
        </section>
      )}
    </>
  );
}

export function ContextPanel({
  context,
  mode,
  propertyGroup,
  years,
  overview,
  onChange,
}: ContextPanelProps) {
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
    queueMicrotask(() => {
      if (!controller.signal.aborted) {
        setLoading(true);
        setError(undefined);
      }
    });
    const request = context.kind === "geography"
      ? Promise.all([
          loadGeography(context.id, controller.signal),
          loadGeographyChildren(context.id, controller.signal),
        ]).then(async ([geography, children]) => ({
          kind: "geography" as const,
          geography,
          children: children.items,
          comparisons: await loadGeographyComparisons(
            mode,
            geography,
            children.items,
            propertyGroup,
            years,
            controller.signal,
          ),
        }))
      : context.kind === "building"
        ? loadBuildingStatistics(
            context.cadastralCode,
            context.buildingNumber,
            propertyGroup,
            years,
            controller.signal,
          ).then((building) => ({ kind: "building" as const, building }))
        : Promise.all([
            loadTransaction(context.mode, context.id, controller.signal),
            loadTransactionContext(context.mode, context.id, controller.signal),
            loadFullValuation(context.mode, context.id, controller.signal)
              .catch(() => undefined),
          ]).then(([detail, transactionContext, model]) => ({
            kind: "transaction" as const,
            detail,
            context: transactionContext,
            model,
          }));
    request.then((result) => {
      if (generation.current !== currentGeneration) return;
      setLoaded(result);
      setLoadedKey(key);
    }).catch((requestError: unknown) => {
      if (controller.signal.aborted || generation.current !== currentGeneration) return;
      setError(requestError instanceof Error ? requestError.message : "Podatkov ni bilo mogoče naložiti.");
    }).finally(() => {
      if (!controller.signal.aborted && generation.current === currentGeneration) setLoading(false);
    });
    return () => controller.abort();
  }, [context, key, mode, propertyGroup, years]);

  useEffect(() => {
    if (context.kind !== "country") return;
    const controller = new AbortController();
    loadMarketIndices(mode, propertyGroup, controller.signal)
      .then((response) => setIndices(response.items))
      .catch(() => { if (!controller.signal.aborted) setIndices([]); });
    return () => controller.abort();
  }, [context.kind, mode, propertyGroup]);

  const current = loadedKey === key ? loaded : undefined;
  const latestIndices = (["hedonic", "matched_pair_median"] as const)
    .map((method) => indices.filter((item) => item.method === method && item.isPublishable).at(-1))
    .filter((item): item is MarketIndex => item != null);
  return (
    <aside className="context-panel" aria-live="polite">
      <header>
        <div>
          <span>Kontekst trga</span>
          <h2>
            {context.kind === "country" ? "Slovenija"
              : context.kind === "geography" ? context.name
                : context.kind === "building" ? context.label ?? `Stavba ${context.buildingNumber}`
                  : `${context.mode === "sale" ? "Prodaja" : "Najem"} #${context.id}`}
          </h2>
        </div>
        {context.kind !== "country" && (
          <button type="button" aria-label="Zapri podrobnosti" onClick={() => onChange({ kind: "country" })}>
            <X size={17} />
          </button>
        )}
      </header>
      {loading && <div className="context-progress">Nalagam posodobljen kontekst …</div>}
      {error && <p className="context-error">{error} Prejšnji uspešni podatki so ostali nespremenjeni.</p>}
      {context.kind === "country" && (
        <>
          <div className="context-primary">
            <span>{mode === "sale" ? "Mediana prodajne cene" : "Mediana mesečne najemnine"}</span>
            <strong>{(mode === "sale"
              ? overview.medianPricePerSqm
              : overview.medianMonthlyRent) == null
              ? "Ni dovolj podatkov"
              : mode === "sale"
                ? `${euro.format(overview.medianPricePerSqm!)}/m²`
                : `${euro.format(overview.medianMonthlyRent!)} / mesec`}</strong>
          </div>
          <p>Izberite regijo, občino, lokalno območje, stavbo ali posel za primerjave na isti ravni.</p>
          {indices.length > 0 && (
            <section className="context-section">
              <h3>Nacionalni indeks cen</h3>
              {latestIndices.map((index) => (
                <div className="benchmark-card" key={index.method}>
                  <div><strong>{index.method === "hedonic" ? "Hedonski" : "Ponovne prodaje"}</strong>
                    <span>baza {index.basePeriod.slice(0, 4)} · n={index.sampleSize}</span>
                  </div>
                  <b>{index.indexValue >= 100 ? "+" : ""}
                    {decimal.format(index.indexValue - 100)} % od {index.basePeriod.slice(0, 4)}</b>
                  <small>indeks {decimal.format(index.indexValue)} · {index.period.slice(0, 7)}</small>
                </div>
              ))}
              <small>Začetno leto je označeno s 100. Indeks 120 pomeni približno 20 % višje cene kot v začetnem letu.</small>
            </section>
          )}
        </>
      )}
      {context.kind !== "country" && !loading && !error && !current && (
        <p>Za izbrani kontekst ni podatkov.</p>
      )}
      {current?.kind === "geography" && (
        <>
          <div className="context-breadcrumbs">
            <button type="button" onClick={() => onChange({ kind: "country" })}>Slovenija</button>
            <ChevronRight size={13} /><span>{current.geography.name}</span>
          </div>
          <p>
            Prikazane statistike, grafi in zemljevid uporabljajo isti filter:
            {` ${propertyGroupLabel(propertyGroup)}, ${years.from}–${years.to}.`}
          </p>
          <section className="context-section">
            <h3>Podrejena območja</h3>
            {current.children.length === 0 ? <p>Ni podrejenih območij.</p> : (
              <div className="context-children">
                {current.children.map((child) => (
                  <button type="button" key={child.id} onClick={() => onChange({
                    kind: "geography",
                    id: child.id,
                    level: child.level,
                    name: child.name,
                  })}>{child.name}</button>
                ))}
              </div>
            )}
          </section>
          {current.comparisons.length > 0 && (
            <section className="context-section">
              <h3>Primerjava podrejenih območij</h3>
              {current.comparisons.map((comparison) => (
                <button className="context-transaction" type="button" key={comparison.id}
                  onClick={() => onChange({
                    kind: "geography",
                    id: comparison.id,
                    level: comparison.level,
                    name: comparison.name,
                  })}>
                  <span>{comparison.name} · n={comparison.representativeCount}</span>
                  <strong>{(mode === "rent" ? comparison.medianAmount : comparison.medianAmountPerSqm) == null
                    ? "mediana ni objavljena"
                    : mode === "rent"
                      ? `${euro.format(comparison.medianAmount!)} / mesec`
                      : `${decimal.format(comparison.medianAmountPerSqm!)} €/m²`}</strong>
                </button>
              ))}
            </section>
          )}
        </>
      )}
      {current?.kind === "building" && (
        <>
          {(() => {
            const summary = mode === "sale" ? current.building.sales : current.building.rentals;
            const history = mode === "sale" ? current.building.saleHistory : current.building.rentalHistory;
            const historyAmount = (point: typeof history[number]) => mode === "sale"
              ? point.medianAmountPerSqm
              : point.medianAmount;
            const max = Math.max(...history.map((point) => historyAmount(point) ?? 0), 1);
            return <>
              <div className="building-market-kpis">
                <div><span>Vsi posli</span><strong>{summary.transactionCount.toLocaleString("sl-SI")}</strong></div>
                <div><span>Reprezentativni</span><strong>{summary.representativeCount.toLocaleString("sl-SI")}</strong></div>
                <div><span>{mode === "sale" ? "Mediana / m²" : "Mesečna mediana"}</span>
                  <strong>{(mode === "sale" ? summary.medianAmountPerSqm : summary.medianAmount) == null
                    ? "—"
                    : mode === "sale"
                      ? `${decimal.format(summary.medianAmountPerSqm!)} €`
                      : euro.format(summary.medianAmount!)}</strong></div>
              </div>
              {history.length > 0 && <section className="context-section">
                <h3>Zgodovina stavbe</h3>
                <div className="building-history">
                  {history.map((point) => <div key={point.year}>
                    <span>{point.year}</span>
                    <i style={{ height: `${Math.max(8, 100 * (historyAmount(point) ?? 0) / max)}%` }} />
                    <strong>{historyAmount(point) == null
                      ? "—"
                      : mode === "sale"
                        ? decimal.format(historyAmount(point)!)
                        : euro.format(historyAmount(point)!)}</strong>
                    <small>n={point.representativeCount}</small>
                  </div>)}
                </div>
              </section>}
            </>;
          })()}
          <FieldList fields={[
            { label: "Katastrska občina", value: current.building.cadastralMunicipalityCode },
            { label: "Številka stavbe", value: current.building.buildingNumber },
            { label: "Leto gradnje", value: current.building.constructionYear },
            { label: "Znani deli", value: current.building.knownBuildingPartCount },
          ]} />
          <section className="context-section">
            <h3>{mode === "sale" ? "Prodaje" : "Najemi"} v stavbi</h3>
            {(mode === "sale"
              ? current.building.saleTransactions
              : current.building.rentalTransactions
            ).map((transaction) => (
              <button className="context-transaction" type="button" key={transaction.transactionId}
                onClick={() => onChange({ kind: "transaction", mode, id: transaction.transactionId })}>
                <span>#{transaction.transactionId} · {transaction.contractDate ?? "datum ni objavljen"}</span>
                <strong>
                  {transaction.amount == null ? "cena ni objavljena" : euro.format(transaction.amount)}
                  {transaction.amountPerSqm == null ? "" : ` · ${decimal.format(transaction.amountPerSqm)} €/m²`}
                </strong>
              </button>
            ))}
          </section>
        </>
      )}
      {current?.kind === "transaction" && (
        <TransactionView loaded={current} onChange={onChange} />
      )}
    </aside>
  );
}
