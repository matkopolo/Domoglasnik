import type { BuildingSearchResult, MarketMode, PropertyGroup } from "./types";

export const propertyGroupLabels: Record<PropertyGroup, string> = {
  apartment: "stanovanja",
  house: "hiše",
  land: "zemljišča",
  commercial: "poslovni prostori",
  garage: "garaže",
  room: "sobe",
};

export function suggestedBuildingMarket(
  result: BuildingSearchResult,
  mode: MarketMode,
  group: PropertyGroup,
) {
  const active = mode === "sale" ? result.saleCounts[group] ?? 0 : result.rentalCounts[group] ?? 0;
  if (active > 0) return undefined;
  const candidates: Array<{ mode: MarketMode; group: PropertyGroup; count: number }> = [];
  for (const [candidateMode, counts] of [["sale", result.saleCounts], ["rent", result.rentalCounts]] as const) {
    for (const [candidateGroup, count] of Object.entries(counts) as Array<[PropertyGroup, number]>) {
      if (count > 0 && !(candidateMode === mode && candidateGroup === group)) {
        candidates.push({ mode: candidateMode, group: candidateGroup, count });
      }
    }
  }
  return candidates.sort((left, right) =>
    Number(right.mode === mode) - Number(left.mode === mode) || right.count - left.count
  )[0];
}
