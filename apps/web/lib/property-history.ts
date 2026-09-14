import type { components } from "./generated/api-schema";
import { getJSON, marketQuery } from "./api";
import type { MarketConstraints, PropertyGroup, YearRange } from "./types";

export type PropertyHistory = components["schemas"]["PropertyHistory"];
export type HistoryAsset = components["schemas"]["HistoryAsset"];
export type ResalePair = components["schemas"]["ResalePair"];
export type ResaleSummary = components["schemas"]["ResaleSummary"];
export type InflationFactor = components["schemas"]["InflationFactor"];

export const loadPropertyHistory = (id: number, signal: AbortSignal) =>
  getJSON<PropertyHistory>(`/api/v1/sales/transactions/${id}/history`, signal);

export const loadResaleSummary = (group: PropertyGroup, years: YearRange, geographyId: number | undefined, area: MarketConstraints, signal: AbortSignal) =>
  getJSON<ResaleSummary>(`/api/v1/sales/statistics/resales?${marketQuery(group, years, geographyId, area)}`, signal);

export const loadInflationFactor = (from: string, to: string, signal: AbortSignal) =>
  getJSON<InflationFactor>(`/api/v1/economy/inflation-factor?${new URLSearchParams({ from, to })}`, signal);

export function googleMapsURL(latitude?: number | null, longitude?: number | null) {
  if (latitude == null || longitude == null || !Number.isFinite(latitude) || !Number.isFinite(longitude)
    || Math.abs(latitude) > 90 || Math.abs(longitude) > 180 || (latitude === 0 && longitude === 0)) return null;
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(`${latitude},${longitude}`)}`;
}

// Evidence only: a geolocation parcel reference is not a cadastral building/parcel
// ownership link. Never call this confirmed construction or use it in resale returns.
export function hasPossibleDevelopment(asset: HistoryAsset) {
  return asset.kind === "parcel" && asset.events.some((house) =>
    house.evidence === "building_geolocation_reference" && house.propertyGroup === "house"
    && house.contractDate != null && house.constructionYear != null
    && house.constructionYear <= Number(house.contractDate.slice(0, 4))
    && asset.events.some((land) => land.evidence === "exact_identity" && land.propertyGroup === "land"
      && land.contractDate != null && land.contractDate < house.contractDate!
      && Number(land.contractDate.slice(0, 4)) < house.constructionYear!));
}
