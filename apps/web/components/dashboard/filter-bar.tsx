"use client";

import { Building2, CalendarDays, MapPin, RotateCcw, SlidersHorizontal, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { propertyGroupLabel } from "@/lib/display-values";
import type { AreaRange, Geography, MarketMode, PropertyGroup, YearRange } from "@/lib/types";

const saleProperties: Array<{ value: PropertyGroup; label: string }> = [
  { value: "apartment", label: "Stanovanja" }, { value: "house", label: "Hiše" },
  { value: "land", label: "Zemljišča" }, { value: "commercial", label: "Poslovni prostori" },
  { value: "garage", label: "Garaže" },
];
const rentalProperties = [
  { value: "apartment" as const, label: "Stanovanja" }, { value: "house" as const, label: "Hiše" },
  { value: "commercial" as const, label: "Poslovni prostori" }, { value: "garage" as const, label: "Garaže" },
  { value: "room" as const, label: "Sobe" },
];

interface FilterBarProps {
  mode: MarketMode; onModeChange: (mode: MarketMode) => void;
  propertyGroup: PropertyGroup; onPropertyChange: (group: PropertyGroup) => void;
  geographyId?: number; onGeographyChange: (id?: number) => void; geographies: Geography[];
  years: YearRange; onYearsChange: (years: YearRange) => void;
  area: AreaRange; onAreaChange: (area: AreaRange) => void;
  onReset?: () => void;
}

export function FilterBar(props: FilterBarProps) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const firstControl = useRef<HTMLSelectElement>(null);
  const selectedGeography = props.geographies.find((item) => item.id === props.geographyId)?.name ?? "Slovenija";
  const areaText = props.area.min != null || props.area.max != null
    ? `${props.area.min ?? 0}–${props.area.max ?? "∞"} m²` : "vse površine";
  const close = () => { setOpen(false); window.setTimeout(() => trigger.current?.focus(), 0); };

  useEffect(() => {
    if (!open) return;
    firstControl.current?.focus();
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") close(); };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  const yearStart = props.mode === "sale" ? 2007 : 2013;
  const yearEnd = new Date().getFullYear();
  const yearOptions = Array.from({ length: yearEnd - yearStart + 1 }, (_, index) => yearStart + index);
  const propertyOptions = props.mode === "sale" ? saleProperties : rentalProperties;

  return <section className="market-context-bar" aria-label="Tržni kontekst in filtri">
    <div className="mobile-market-summary">
      <div><strong>{props.mode === "sale" ? "Prodaja" : "Najem"} · {propertyGroupLabel(props.propertyGroup)}</strong>
        <span>{selectedGeography}</span><small>{props.years.from}–{props.years.to} · {areaText}</small></div>
      <button ref={trigger} type="button" aria-expanded={open} aria-controls="market-filter-panel" onClick={() => setOpen(true)}>
        <SlidersHorizontal aria-hidden="true" /> Spremeni filtre
      </button>
    </div>
    {open && <button type="button" className="filter-backdrop" aria-label="Zapri filtre" onClick={close} />}
    <div id="market-filter-panel" className={`filter-panel ${open ? "open" : ""}`}
      role={open ? "dialog" : undefined} aria-modal={open || undefined} aria-label={open ? "Spremeni filtre" : undefined}>
      <div className="filter-panel-heading"><div><span>Tržni kontekst</span><strong>Izberite analizirano populacijo</strong></div>
        <button type="button" className="icon-button filter-close" aria-label="Zapri filtre" onClick={close}><X /></button></div>
      <div className="primary-filter-group">
        <div className="mode-switch" role="group" aria-label="Vrsta posla">
          <button type="button" className={props.mode === "sale" ? "active" : ""} onClick={() => props.onModeChange("sale")}>Prodaja</button>
          <button type="button" className={props.mode === "rent" ? "active" : ""} onClick={() => props.onModeChange("rent")}>Najem</button>
        </div>
        <label className="filter-field primary"><Building2 aria-hidden="true" /><span>Nepremičnina</span>
          <select ref={firstControl} aria-label="Vrsta nepremičnine" value={props.propertyGroup}
            onChange={(event) => props.onPropertyChange(event.target.value as PropertyGroup)}>
            {propertyOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select></label>
        <label className="filter-field primary"><MapPin aria-hidden="true" /><span>Območje</span>
          <select aria-label="Območje" value={props.geographyId ?? ""}
            onChange={(event) => props.onGeographyChange(event.target.value ? Number(event.target.value) : undefined)}>
            <option value="">Slovenija</option>
            {props.geographies.map((region) => <option key={region.id} value={region.id}>{region.name}</option>)}
          </select></label>
      </div>
      <div className="secondary-filter-group">
        <label className="filter-field range-field"><CalendarDays aria-hidden="true" /><span>Obdobje</span>
          <select aria-label="Od leta" value={props.years.from} onChange={(event) => props.onYearsChange({
            from: Number(event.target.value), to: Math.max(Number(event.target.value), props.years.to),
          })}>{yearOptions.map((year) => <option key={year} value={year}>{year}</option>)}</select><b aria-hidden="true">–</b>
          <select aria-label="Do leta" value={props.years.to} onChange={(event) => props.onYearsChange({
            from: Math.min(props.years.from, Number(event.target.value)), to: Number(event.target.value),
          })}>{yearOptions.map((year) => <option key={year} value={year}>{year}</option>)}</select></label>
        <label className="filter-field range-field"><SlidersHorizontal aria-hidden="true" /><span>Površina</span>
          <input aria-label="Površina od" type="number" min="1" step="1" placeholder="Od m²" value={props.area.min ?? ""}
            onChange={(event) => props.onAreaChange({ ...props.area, min: event.target.value ? Number(event.target.value) : undefined })}/><b aria-hidden="true">–</b>
          <input aria-label="Površina do" type="number" min="1" step="1" placeholder="Do m²" value={props.area.max ?? ""}
            onChange={(event) => props.onAreaChange({ ...props.area, max: event.target.value ? Number(event.target.value) : undefined })}/></label>
        {props.onReset && <button type="button" className="reset-filters" onClick={props.onReset}><RotateCcw aria-hidden="true" /> Ponastavi</button>}
      </div>
      <div className="active-filter-summary" aria-live="polite"><strong>{props.mode === "sale" ? "Prodaja" : "Najem"} · {propertyGroupLabel(props.propertyGroup)}</strong>
        <span>{selectedGeography} · {props.years.from}–{props.years.to} · {areaText}</span></div>
      <button type="button" className="apply-filters" onClick={close}>Prikaži izbrani trg</button>
    </div>
  </section>;
}
