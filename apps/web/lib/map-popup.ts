import { decimal, euro } from "@/lib/display-values";
import type { MapPointFeature, MarketMode } from "@/lib/types";

export function mapAmountLabel(
  properties: MapPointFeature["properties"],
  mode: MarketMode,
) {
  const value = mode === "sale"
    ? properties.medianPricePerSqm ?? properties.pricePerSqm
    : properties.medianMonthlyRent ?? properties.monthlyRent;
  return typeof value === "number"
    ? mode === "sale" ? `${decimal.format(value)} €/m²` : `${euro.format(value)} / mesec`
    : undefined;
}

export function buildMapPopup(feature: MapPointFeature, mode: MarketMode) {
  const wrapper = document.createElement("div");
  wrapper.className = "map-popup compact";
  const properties = feature.properties;
  const kind = String(properties.featureKind);
  const title = document.createElement("strong");
  title.textContent = kind === "building"
    ? `Stavba · ${Number(properties.transactionCount ?? 1)} poslov`
    : kind === "property"
      ? `${mode === "sale" ? "Prodaja" : "Najem"} #${properties.transactionId ?? ""}`
      : String(properties.name ?? `${properties.transactionCount ?? 0} poslov`);
  wrapper.append(title);
  const amount = mapAmountLabel(properties, mode);
  if (amount) {
    const row = document.createElement("span");
    row.textContent = amount;
    wrapper.append(row);
  }
  if (kind === "property" && typeof properties.area === "number") {
    const row = document.createElement("span");
    row.textContent = `${decimal.format(properties.area)} m²`;
    wrapper.append(row);
  }
  if (kind === "property" && mode === "sale" && typeof properties.price === "number") {
    const row = document.createElement("span");
    row.textContent = euro.format(properties.price);
    wrapper.append(row);
  }
  const hint = document.createElement("small");
  hint.textContent = "Podrobnosti so odprte v kontekstnem panelu.";
  wrapper.append(hint);
  return wrapper;
}
