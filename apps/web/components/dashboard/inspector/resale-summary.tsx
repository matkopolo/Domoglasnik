"use client";

import { useEffect, useState } from "react";
import { loadResaleSummary, type ResaleSummary as Summary } from "@/lib/property-history";
import { decimal } from "@/lib/display-values";
import type { MarketConstraints, PropertyGroup, YearRange } from "@/lib/types";
import styles from "./property-history.module.css";

export function ResaleSummary({ geographyId, propertyGroup, years, area, name = "izbrano območje" }: {
  geographyId?: number; propertyGroup: PropertyGroup; years: YearRange; area: MarketConstraints; name?: string;
}) {
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<{ key: string; value?: Summary; error?: boolean }>();
  const key = JSON.stringify([geographyId, propertyGroup, years, area]);
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    loadResaleSummary(propertyGroup, years, geographyId, area, controller.signal)
      .then((value) => { if (!controller.signal.aborted) setState({ key, value }); })
      .catch(() => { if (!controller.signal.aborted) setState({ key, error: true }); });
    return () => controller.abort();
  }, [open, key, geographyId, propertyGroup, years, area]);
  const current = state?.key === key ? state : undefined;
  return <div className={styles.history}><details onToggle={(event) => setOpen(event.currentTarget.open)}>
    <summary>Čas med ponovnimi prodajami</summary>
    <p><strong>Območje: {name}</strong></p>
    <p>Med opazovanimi primerljivimi ponovnimi prodajami, ne med vsemi lastniki. Filtri veljajo za poznejšo prodajo; prva je lahko pred izbranim obdobjem.</p>
    {open && !current && <p role="status">Izračunavam intervale …</p>}
    {current?.error && <p role="alert">Statistike ponovnih prodaj ni bilo mogoče naložiti.</p>}
    {current?.value && <><p>{current.value.pairCount.toLocaleString("sl-SI")} parov · {current.value.assetCount.toLocaleString("sl-SI")} nepremičnin.</p>
      {current.value.isPublishable ? <dl className={styles.metadata}>
        <div><dt>P25</dt><dd>{decimal.format(current.value.p25Days!)} dni</dd></div>
        <div><dt>Mediana</dt><dd>{decimal.format(current.value.medianDays!)} dni</dd></div>
        <div><dt>P75</dt><dd>{decimal.format(current.value.p75Days!)} dni</dd></div>
      </dl> : <p>{current.value.pairCount === 0 ? "Ni ustreznih parov." : "Premalo podatkov za objavo percentilov; potrebujemo najmanj 30 parov."}</p>}
      <small>Enaka katastrska identiteta, ena sestavina, znan celotni delež, reprezentativni prodaji, enaka vrsta in površina ±1 %, brez možnih dvojnikov.
        Kratke ponovne prodaje niso samodejno izločene. Dedovanja, darila in nepopolni zapisi niso zajeti.</small>
    </>}
  </details></div>;
}
