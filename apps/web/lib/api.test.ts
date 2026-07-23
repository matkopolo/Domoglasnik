import { afterEach, describe, expect, it, vi } from "vitest";
import { loadMapFeatures } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("loadMapFeatures", () => {
  it("requests zoom-aware sale clusters for broad map views", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      type: "FeatureCollection",
      features: [],
    }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await loadMapFeatures(
      "sale",
      "apartment",
      undefined,
      { from: 2018, to: 2025 },
      [13.3, 45.4, 16.7, 46.9],
      8.2,
      new AbortController().signal,
    );

    const url = String(fetchMock.mock.calls[0][0]);
    expect(url).toContain("/api/v1/map/geographies?");
    expect(url).toContain("bbox=13.3%2C45.4%2C16.7%2C46.9");
    expect(url).toContain("level=statistical_region");
  });

  it("requests individual rental properties at street level", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      type: "FeatureCollection",
      features: [{
        type: "Feature",
        id: 42,
        geometry: { type: "Point", coordinates: [14.5, 46.1] },
        properties: { transactionId: 42 },
      }],
    }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await loadMapFeatures(
      "rent",
      "apartment",
      9,
      { from: 2018, to: 2025 },
      [14.3, 46, 14.7, 46.3],
      14,
      new AbortController().signal,
    );

    const url = String(fetchMock.mock.calls[0][0]);
    expect(url).toContain("/api/v1/rentals/map/properties?");
    expect(url).toContain("geographyId=9");
    expect(url).toContain("limit=500");
    expect(result.features[0].properties.featureKind).toBe("property");
  });

  it("consolidates sale transactions from the same building", async () => {
    const feature = (transactionId: number) => ({
      type: "Feature",
      id: transactionId,
      geometry: { type: "Point", coordinates: [15.61, 46.53] },
      properties: {
        transactionId,
        cadastralMunicipalityCode: 678,
        buildingNumber: "2952",
      },
    });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
      type: "FeatureCollection",
      features: [feature(1), feature(2)],
    }), { status: 200 })));

    const result = await loadMapFeatures(
      "sale",
      "apartment",
      undefined,
      { from: 2018, to: 2025 },
      [15.5, 46.5, 15.7, 46.6],
      14,
      new AbortController().signal,
    );

    expect(result.features).toHaveLength(1);
    expect(result.features[0].properties.featureKind).toBe("building");
    expect(result.features[0].properties.transactionCount).toBe(2);
  });
});
