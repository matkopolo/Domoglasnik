import { describe, expect, it } from "vitest";
import { decimal } from "@/lib/display-values";
import { buildMapPopup, mapAmountLabel } from "@/lib/map-popup";
import type { MapPointFeature } from "@/lib/types";

const sale: MapPointFeature = {
  type: "Feature",
  geometry: { type: "Point", coordinates: [15.6, 46.5] },
  properties: {
    featureKind: "property",
    transactionId: 42,
    pricePerSqm: 2500,
    area: 50,
    price: 125000,
  },
};

describe("map popup presentation", () => {
  it("keeps the established sale detail fields", () => {
    const popup = buildMapPopup(sale, "sale");

    expect(popup.textContent).toContain("Prodaja #42");
    expect(popup.textContent).toContain(`${decimal.format(2500)} €/m²`);
    expect(popup.textContent).toContain("50 m²");
    expect(popup.textContent).toContain("125.000");
  });

  it("prefers aggregate median values", () => {
    expect(mapAmountLabel({
      featureKind: "cluster",
      medianPricePerSqm: 3100,
      pricePerSqm: 2000,
    }, "sale")).toBe(`${decimal.format(3100)} €/m²`);
  });
});
