import { describe, expect, it } from "vitest";
import { suggestedBuildingMarket } from "./building-search";
import type { BuildingSearchResult } from "./types";

const building: BuildingSearchResult = {
  cadastralMunicipalityCode: 1712,
  buildingNumber: "727",
  settlement: "Zapotok",
  street: "",
  houseNumber: "27",
  longitude: 14.5,
  latitude: 46.1,
  saleCount: 4,
  rentalCount: 2,
  saleCounts: { house: 4 },
  rentalCounts: { apartment: 2 },
};

describe("suggestedBuildingMarket", () => {
  it("prefers another property group in the active transaction mode", () => {
    expect(suggestedBuildingMarket(building, "sale", "apartment")).toEqual({
      mode: "sale", group: "house", count: 4,
    });
  });

  it("does not suggest a switch when the active market has transactions", () => {
    expect(suggestedBuildingMarket(building, "sale", "house")).toBeUndefined();
  });
});
