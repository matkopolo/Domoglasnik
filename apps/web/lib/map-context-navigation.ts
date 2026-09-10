import type { Map as LibreMap } from "maplibre-gl";
import { loadBuildingStatistics, loadGeography } from "./api";
import type { MarketContext, PropertyGroup, YearRange } from "./types";

export async function focusAreaContext(
  map: Pick<LibreMap, "fitBounds" | "easeTo">,
  context: MarketContext, propertyGroup: PropertyGroup, years: YearRange, signal: AbortSignal,
): Promise<string | undefined> {
  signal.throwIfAborted();
  if (context.kind === "country") {
    map.fitBounds([[13.25, 45.35], [16.75, 46.95]], { padding: 20, duration: 650 });
  } else if (context.kind === "geography") {
    const geography = await loadGeography(context.id, signal);
    signal.throwIfAborted();
    if (!geography.bounds) return "Meje izbranega območja niso na voljo.";
    const [west, south, east, north] = geography.bounds;
    map.fitBounds([[west, south], [east, north]], { padding: 42, maxZoom: 13.4, duration: 700 });
  } else if (context.kind === "building") {
    const building = await loadBuildingStatistics(context.cadastralCode, context.buildingNumber, propertyGroup, years, signal);
    signal.throwIfAborted();
    if (building.longitude == null || building.latitude == null) return "Stavba nima uporabnih koordinat.";
    map.easeTo({ center: [building.longitude, building.latitude], zoom: 17, duration: 700 });
  }
}
