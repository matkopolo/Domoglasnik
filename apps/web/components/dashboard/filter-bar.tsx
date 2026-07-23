import { Building2, CalendarDays, MapPin, SlidersHorizontal } from "lucide-react";
import type { Geography, MarketMode, PropertyGroup, YearRange } from "@/lib/types";

const properties: Array<{ value: PropertyGroup; label: string }> = [
  { value: "apartment", label: "Stanovanja" },
  { value: "house", label: "Hiše" },
  { value: "land", label: "Zemljišča" },
  { value: "commercial", label: "Poslovni prostori" },
  { value: "garage", label: "Garaže" },
];

interface FilterBarProps {
  mode: MarketMode;
  onModeChange: (mode: MarketMode) => void;
  propertyGroup: PropertyGroup;
  onPropertyChange: (group: PropertyGroup) => void;
  geographyId?: number;
  onGeographyChange: (id?: number) => void;
  geographies: Geography[];
  years: YearRange;
  onYearsChange: (years: YearRange) => void;
}

export function FilterBar(props: FilterBarProps) {
  return (
    <section className="filter-bar" aria-label="Filtri trga">
      <div className="mode-switch" role="group" aria-label="Vrsta posla">
        <button className={props.mode === "sale" ? "active" : ""} onClick={() => props.onModeChange("sale")}>
          Prodaja
        </button>
        <button className={props.mode === "rent" ? "active rental" : ""} onClick={() => props.onModeChange("rent")}>
          Najem
        </button>
      </div>
      <label className="filter-field">
        <MapPin size={17} />
        <span className="sr-only">Območje</span>
        <select
          value={props.geographyId ?? ""}
          onChange={(event) => props.onGeographyChange(event.target.value ? Number(event.target.value) : undefined)}
        >
          <option value="">Slovenija</option>
          {props.geographies.map((region) => <option key={region.id} value={region.id}>{region.name}</option>)}
        </select>
      </label>
      <label className="filter-field date-field">
        <CalendarDays size={17} />
        <span className="sr-only">Od leta</span>
        <select
          aria-label="Od leta"
          value={props.years.from}
          onChange={(event) => props.onYearsChange({
            from: Number(event.target.value),
            to: Math.max(Number(event.target.value), props.years.to),
          })}
        >
          {Array.from({ length: 2026 - (props.mode === "sale" ? 2007 : 2013) + 1 }, (_, index) => (
            <option key={index} value={(props.mode === "sale" ? 2007 : 2013) + index}>
              {(props.mode === "sale" ? 2007 : 2013) + index}
            </option>
          ))}
        </select>
        <span>–</span>
        <span className="sr-only">Do leta</span>
        <select
          aria-label="Do leta"
          value={props.years.to}
          onChange={(event) => props.onYearsChange({
            from: Math.min(props.years.from, Number(event.target.value)),
            to: Number(event.target.value),
          })}
        >
          {Array.from({ length: 2026 - (props.mode === "sale" ? 2007 : 2013) + 1 }, (_, index) => (
            <option key={index} value={(props.mode === "sale" ? 2007 : 2013) + index}>
              {(props.mode === "sale" ? 2007 : 2013) + index}
            </option>
          ))}
        </select>
      </label>
      <label className="filter-field">
        <Building2 size={17} />
        <span className="sr-only">Vrsta nepremičnine</span>
        <select value={props.propertyGroup} onChange={(event) => props.onPropertyChange(event.target.value as PropertyGroup)}>
          {properties.filter((item) => props.mode === "sale" || item.value !== "land").map((item) => (
            <option key={item.value} value={item.value}>{item.label}</option>
          ))}
        </select>
      </label>
      <button className="more-filters" type="button" title="Dodatni filtri bodo dodani v naslednji različici">
        <SlidersHorizontal size={17} />
        <span>Več filtrov</span>
      </button>
    </section>
  );
}
