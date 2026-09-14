"use client";

import { useEffect, useState } from "react";
import { ProvenanceBadge } from "@/components/ui/provenance-badge";
import { decimal, euro, propertyGroupLabel } from "@/lib/display-values";
import { hasPossibleDevelopment, loadInflationFactor, loadPropertyHistory, type PropertyHistory as History, type ResalePair, type InflationFactor } from "@/lib/property-history";
import type { MarketContext } from "@/lib/types";
import { ResaleScenario } from "./resale-scenario";
import styles from "./property-history.module.css";

function InflationChange({ pair }: { pair: ResalePair }) {
  const [opened, setOpened] = useState(false);
  const [result, setResult] = useState<InflationFactor>();
  const [error, setError] = useState(false);
  useEffect(() => {
    if (!opened || !pair.from || !pair.to) return;
    const controller = new AbortController();
    loadInflationFactor(pair.from, pair.to, controller.signal)
      .then((value) => { if (!controller.signal.aborted) setResult(value); })
      .catch(() => { if (!controller.signal.aborted) setError(true); });
    return () => controller.abort();
  }, [opened, pair.from, pair.to]);
  return <details onToggle={(event) => setOpened(event.currentTarget.open)}>
    <summary>Sprememba po inflaciji</summary>
    {opened && !result && !error && <p role="status">Nalagam mesečne indekse SURS …</p>}
    {error && <p role="alert">Popolna mesečna serija ni na voljo. Prilagoditev ni izračunana.</p>}
    {result && pair.previousPrice != null && pair.price != null && <>
      <p>Prejšnja cena v cenah meseca druge prodaje: {euro.format(pair.previousPrice * result.factor)}.</p>
      <p>Razlika po inflaciji: {euro.format(pair.price - pair.previousPrice * result.factor)} ({decimal.format(100 * (pair.price / (pair.previousPrice * result.factor) - 1))} %).</p>
      <small>{result.source} · {result.from.slice(0, 7)}–{result.to.slice(0, 7)} · faktor {result.factor.toFixed(4)}.
        Približek iz zaokroženih mesečnih indeksov, ne dnevni indeks ali neto dobiček.</small>
    </>}
  </details>;
}

export function PropertyHistory({ transactionId, onChange }: { transactionId: number; onChange: (context: MarketContext) => void }) {
  return <PropertyHistoryContent key={transactionId} transactionId={transactionId} onChange={onChange}/>;
}

function PropertyHistoryContent({ transactionId, onChange }: { transactionId: number; onChange: (context: MarketContext) => void }) {
  const [history, setHistory] = useState<History>();
  const [error, setError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    loadPropertyHistory(transactionId, controller.signal)
      .then((value) => { if (!controller.signal.aborted) setHistory(value); })
      .catch(() => { if (!controller.signal.aborted) setError(true); });
    return () => controller.abort();
  }, [transactionId]);
  return <section className={styles.history} aria-label="Zgodovina nepremičnine">
    <h3>Zgodovina nepremičnine</h3><ProvenanceBadge kind="source"/>
    <p>Evidentirani posli z istim katastrskim identifikatorjem v celotni razpoložljivi časovnici, ne le v izbranih letih.
      To ni lastniška zgodovina: darila, dedovanje in spremembe parcel lahko prekinejo povezavo.</p>
    {error ? <p role="alert">Zgodovine ni bilo mogoče naložiti.</p> : !history ? <p role="status">Iščem pretekle posle …</p> : <>
      {history.truncated && <p className={styles.warning}>Prikazanih je prvih 50 identifikatorjev. Paket je obsežnejši.</p>}
      {!history.assets.length && <p>Za ta posel ni dovolj popolnih katastrskih identifikatorjev za zanesljivo povezovanje.</p>}
      {history.assets.map((asset) => <details key={`${asset.kind}:${asset.cadastralCode}:${asset.number}:${asset.partNumber}`} open>
        <summary>{asset.kind === "parcel" ? "Parcela" : "Del stavbe"} · k. o. {asset.cadastralCode} · {asset.number}{asset.partNumber ? ` / del ${asset.partNumber}` : ""} · {asset.events.length} zapisov</summary>
        {asset.truncated && <p className={styles.warning}>Prvih 200 zapisov; časovnica je skrajšana in primerjave niso izračunane.</p>}
        {hasPossibleDevelopment(asset) && <p className={styles.warning}>Možna poznejša pozidava: po evidentirani prodaji zemljišča obstaja hišni posel z isto geolokacijsko parcelno referenco in poznejšim letom gradnje. Povezava ni dokaz gradnje ali lastništva parcele.</p>}
        <ol className={styles.timeline}>{asset.events.map((event) => <li key={`${event.transactionId}:${event.evidence}:${event.componentId}`}>
          <button type="button" onClick={() => onChange({ kind: "transaction", mode: "sale", id: event.transactionId })}>
            {event.contractDate ?? "Datum ni znan"} · Posel #{event.transactionId} →</button>
          <p>{event.evidence === "exact_identity" ? "Enak katastrski identifikator" : "Samo geolokacijska referenca stavbe"} · {propertyGroupLabel(event.propertyGroup)}</p>
          <dl className={styles.metadata}>
            <div><dt>Cena celotnega posla</dt><dd>{event.agreementPrice == null ? "Ni na voljo" : euro.format(event.agreementPrice)}</dd></div>
            <div><dt>Pogodbena cena sestavine</dt><dd>{event.componentPrice == null ? "Ni na voljo" : euro.format(event.componentPrice)}</dd></div>
            <div><dt>Površina sestavine</dt><dd>{event.area == null ? "Ni na voljo" : `${decimal.format(event.area)} m²`}</dd></div>
            <div><dt>Prodani delež</dt><dd>{event.share ?? "Ni znan"}</dd></div>
            <div><dt>{asset.kind === "parcel" && event.evidence === "exact_identity" ? "Vrsta zemljišča ETN" : "Vrsta dela stavbe ETN"}</dt><dd>{event.sourceType ?? "Ni znana"}</dd></div>
            <div><dt>Leto gradnje</dt><dd>{event.constructionYear ?? "Ni znano"}</dd></div>
          </dl>
          <p>{event.representative ? "Reprezentativen posel" : "Posel ni v reprezentativnem vzorcu"}{event.possibleDuplicate ? " · Možen podvojen zapis" : ""}.</p>
          {event.componentCount > 1 && <p className={styles.warning}>Paket vsebuje {event.componentCount} sestavin. Skupna cena ni cena samo te nepremičnine.</p>}
        </li>)}</ol>
        {asset.pairs.map((pair) => <div className={styles.pair} key={`${pair.previousId}:${pair.transactionId}`}>
          <h4>#{pair.previousId} → #{pair.transactionId}</h4>
          <p>Čas med evidentiranima prodajama: {pair.days == null || pair.days < 0 ? "ni izračunan" : `${pair.days.toLocaleString("sl-SI")} dni`}. Ni nujno obdobje lastništva.</p>
          {pair.landTransition && <p className={styles.warning}>Sprememba evidentirane vrste zemljišča: kmetijsko (ETN 7/8) → zazidljivo (ETN 1/2). Ne potrjuje pravne spremembe namenske rabe ali njenega datuma.</p>}
          {pair.comparable ? <><ProvenanceBadge kind="calculated"/><p>Razlika med prodajnima cenama: {euro.format(pair.nominalChange!)} · {decimal.format(pair.changePercent!)} %. To ni dejanski dobiček.</p>
            <InflationChange key={`${pair.previousId}:${pair.transactionId}`} pair={pair}/></>
            : <p>Primerjava cen ni objavljena: zahtevamo dve reprezentativni prodaji ene sestavine, znan celotni delež, enako vrsto, površino znotraj ±1 % in brez oznake možnega dvojnika. Preverite zgornje podatke; to ni dokaz napake.</p>}
        </div>)}
        {asset.pairs.length === 0 && !asset.truncated && <p>Ni dveh evidentiranih prodaj iste sestavine za primerjavo.</p>}
      </details>)}
    </>}
    <details><summary>Scenarij stroškov in davkov</summary><ResaleScenario/></details>
  </section>;
}
