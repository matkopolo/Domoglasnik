import { afterEach, describe, expect, it, vi } from "vitest";
import { APIError, loadDashboard, loadGeographyComparisons, loadMapFeatures, marketQuery } from "./api";

afterEach(() => vi.unstubAllGlobals());

it("adds inclusive area bounds to every market query", () => {
  const query = marketQuery("apartment", { from: 2025, to: 2025 }, 12, { min: 60, max: 90 });
  expect(query.get("minArea")).toBe("60");
  expect(query.get("maxArea")).toBe("90");
});

describe("loadMapFeatures", () => {
  it("requests zoom-aware sale clusters for broad map views", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      void input;
      return new Response(JSON.stringify({
        type: "FeatureCollection",
        features: [],
      }), { status: 200 });
    });
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
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      void input;
      return new Response(JSON.stringify({
        type: "FeatureCollection",
        features: [{
          type: "Feature",
          id: 42,
          geometry: { type: "Point", coordinates: [14.5, 46.1] },
          properties: { transactionId: 42 },
        }],
      }), { status: 200 });
    });
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

describe("loadDashboard", () => {
  it("surfaces an API failure instead of returning demo data", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("unavailable", { status: 503 })));
    await expect(loadDashboard(
      "sale",
      "apartment",
      undefined,
      { from: 2024, to: 2025 },
      new AbortController().signal,
    )).rejects.toBeInstanceOf(APIError);
  });
});

describe("loadGeographyComparisons", () => {
  it.each([
    ["sale", "medianPricePerSqm", 1430],
    ["rent", "medianMonthlyRentPerSqm", 12.6],
  ] as const)("maps the %s mode-specific median", async (mode, field, median) => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
      type: "FeatureCollection",
      features: [{
        type: "Feature",
        id: 137,
        geometry: { type: "Point", coordinates: [15.63, 46.56] },
        properties: {
          geographyId: 137,
          level: "municipality",
          name: "Maribor",
          transactionCount: 10,
          representativeCount: 8,
          [field]: median,
        },
      }],
    }), { status: 200 })));

    const result = await loadGeographyComparisons(
      mode,
      {
        id: 2,
        level: "statistical_region",
        name: "Podravska",
        bounds: [15.3, 46.2, 16.3, 46.8],
      },
      [{ id: 137, level: "municipality", name: "Maribor" }],
      "apartment",
      { from: 2018, to: 2025 },
      new AbortController().signal,
    );

    expect(result[0].medianAmountPerSqm).toBe(median);
  });
});
