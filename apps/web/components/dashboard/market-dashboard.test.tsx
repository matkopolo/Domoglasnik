import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MarketDashboard } from "./market-dashboard";

const { loadDashboardMock } = vi.hoisted(() => ({ loadDashboardMock: vi.fn() }));
vi.mock("@/lib/api", async () => {
  return {
    loadDashboard: loadDashboardMock,
    loadDataStatus: vi.fn(async () => ({
      status: "current",
      activeRelease: {
        id: "active", status: "published", stage: "published",
        startedAt: "2026-04-01T08:00:00Z", publishedAt: "2026-04-01T09:00:00Z",
        analyticsSaleThrough: "2026-03-31", analyticsRentThrough: "2026-03-31",
      },
    })),
    loadMarketIndices: vi.fn(async () => ({ items: [] })),
    loadDetailedTimeSeries: vi.fn(async () => ({ items: [] })),
    loadInflation: vi.fn(async () => ({ items: [], source: "SURS SiStat H281S" })),
    loadStatisticalOutliers: vi.fn(async () => ({ items: [], total: 0 })),
    loadAnomalySummary: vi.fn(async () => ({ total: 0, flagged: 0, unreviewed: 0, highPriority: 0 })),
    loadTransactionExtremes: vi.fn(async () => ({ items: [], population: "representative" })),
    loadGeographyRankings: vi.fn(async () => []),
    loadGeography: vi.fn(async (id: number) => ({
      id, name: "Podravska", level: "statistical_region",
    })),
    loadGeographyChildren: vi.fn(async () => ({ items: [] })),
    loadGeographyComparisons: vi.fn(async () => []),
    loadBuildingStatistics: vi.fn(async () => ({
      cadastralMunicipalityCode: 665, buildingNumber: "8", knownBuildingPartCount: 2,
      sales: { transactionCount: 2, representativeCount: 2 },
      rentals: { transactionCount: 0, representativeCount: 0 },
      saleHistory: [], rentalHistory: [],
      saleTransactions: [{ transactionId: 501, propertyGroup: "apartment" }],
      rentalTransactions: [],
    })),
    loadTransaction: vi.fn(async () => ({
      transactionId: 501, datasetYear: 2023, propertyGroup: "apartment",
      priceDerivation: "transaction", quality: {
        isQualityEligible: true, isRepresentative: true, reasons: [],
      }, geographies: [
        { id: 12, level: "statistical_region", name: "Podravska" },
        { id: 77, level: "municipality", name: "Ruše" },
      ], buildingParts: [], landParcels: [],
    })),
    loadTransactionContext: vi.fn(async () => ({
      transactionKind: "sale", transactionId: 501, contractYear: 2023,
      incompletePeriod: false, propertyGroup: "apartment", actualAmountPerSqm: 2500,
      directComparables: [],
      benchmarks: [], quality: {
        isQualityEligible: true, isRepresentative: true, reasons: [],
      }, methodologyVersion: "test",
    })),
    loadFullValuation: vi.fn(async () => undefined),
  };
});

vi.mock("./slovenia-map", () => ({
  SloveniaMap: ({ onContextChange }: {
    onContextChange: (context: {
      kind: "geography"; id: number; level: "statistical_region"; name: string;
    } | { kind: "transaction"; mode: "sale"; id: number }) => void;
  }) => <div role="application" aria-label="Interaktivni zemljevid nepremičninskih poslov v Sloveniji">
    <input id="building-address-search" aria-label="Naslov stavbe" />
    <button type="button" onClick={() => onContextChange({
      kind: "geography", id: 12, level: "statistical_region", name: "Podravska",
    })}>Izberi Podravsko</button>
    <button type="button" onClick={() => onContextChange({
      kind: "transaction", mode: "sale", id: 501,
    })}>Odpri posel</button>
  </div>,
}));

afterEach(cleanup);

describe("MarketDashboard", () => {
  beforeEach(async () => {
    const { demoDashboard } = await import("@/lib/demo-data");
    loadDashboardMock.mockReset();
    loadDashboardMock.mockImplementation(async (mode: "sale" | "rent") => demoDashboard(mode));
    Element.prototype.scrollIntoView = vi.fn();
    window.history.replaceState(null, "", "/");
  });

  it("renders the map-first market overview", async () => {
    render(<MarketDashboard />);
    expect(screen.getByRole("heading", { name: "Je cena nepremičnine realna?" })).toBeInTheDocument();
    expect(screen.getByRole("application", { name: "Interaktivni zemljevid nepremičninskih poslov v Sloveniji" })).toBeInTheDocument();
    const metrics = await screen.findAllByText("Mediana cene / m²");
    expect(metrics.length).toBeGreaterThan(0);
    expect(screen.getAllByText(/2[.\s]?596 €\/m²/).length).toBeGreaterThan(0);
    expect(screen.getByText(/Geodetska uprava Republike Slovenije/)).toBeInTheDocument();
    expect(screen.getByText(/GURS te aplikacije ne podpira/)).toBeInTheDocument();
    expect(screen.getByText(/niso uradna ali certificirana cenitev/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "CC BY 4.0" })).toHaveAttribute(
      "href",
      "https://creativecommons.org/licenses/by/4.0/deed.sl",
    );
  });

  it("takes the buyer directly to address search", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: /Preverite konkreten naslov/ }));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Naslov stavbe" })).toHaveFocus(), {
      timeout: 600,
    });
  });

  it("returns to the default Slovenian market from the brand", async () => {
    window.history.replaceState(null, "", "/?mode=rent&propertyGroup=house&from=2022&to=2024&context=country");
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: /DOMOGLASNIK — začetni pogled/ }));
    await waitFor(() => {
      expect(window.location.search).toContain("mode=sale");
      expect(window.location.search).toContain("propertyGroup=apartment");
      expect(window.location.search).toContain("from=2018");
      expect(window.location.search).toContain("to=2025");
      expect(window.location.search).toContain("context=country");
    });
  });

  it("switches to rental metrics", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    const metrics = await screen.findAllByText("Mediana mesečne najemnine");
    expect(metrics.some((metric) => metric.nextElementSibling?.textContent?.includes("650"))).toBe(true);
  });

  it("allows the analysis year range to be changed", () => {
    render(<MarketDashboard />);
    const from = screen.getByRole("combobox", { name: "Od leta" });
    const to = screen.getByRole("combobox", { name: "Do leta" });
    fireEvent.change(from, { target: { value: "2020" } });
    fireEvent.change(to, { target: { value: "2024" } });
    expect(from).toHaveValue("2020");
    expect(to).toHaveValue("2024");
    expect(window.location.search).toContain("from=2020");
    expect(window.location.search).toContain("to=2024");
  });

  it("exposes all land including agricultural land and persists the scope in the URL", async () => {
    render(<MarketDashboard />);
    fireEvent.change(screen.getByRole("combobox", { name: "Vrsta nepremičnine" }), { target: { value: "land" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Vrste zemljišč" }), { target: { value: "all" } });
    await waitFor(() => expect(window.location.search).toContain("landScope=all"));
    expect(loadDashboardMock.mock.calls.at(-1)?.[5]).toMatchObject({ landScope: "all" });
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    expect(screen.queryByRole("combobox", { name: "Vrste zemljišč" })).not.toBeInTheDocument();
    expect(window.location.search).not.toContain("landScope");
  });

  it.each(["agricultural", "forest", "mixed", "unverified", "other", "broad"])("restores the %s land population from a shared URL", async (scope) => {
    window.history.replaceState(null, "", `/?mode=sale&propertyGroup=land&landScope=${scope}&from=2024&to=2025&context=country`);
    render(<MarketDashboard/>);
    expect(screen.getByRole("combobox", { name: "Vrste zemljišč" })).toHaveValue(scope);
    await waitFor(() => expect(loadDashboardMock.mock.calls.at(-1)?.[5]).toMatchObject({ landScope: scope }));
    expect(window.location.search).toContain(`landScope=${scope}`);
  });

  it("filters the market by an inclusive area range", async () => {
    render(<MarketDashboard />);
    fireEvent.change(screen.getByRole("spinbutton", { name: "Površina od" }), {
      target: { value: "60" },
    });
    fireEvent.change(screen.getByRole("spinbutton", { name: "Površina do" }), {
      target: { value: "90" },
    });
    await waitFor(() => {
      expect(loadDashboardMock.mock.calls.at(-1)?.[5]).toEqual({ min: 60, max: 90 });
    });
    expect(window.location.search).toContain("minArea=60");
    expect(window.location.search).toContain("maxArea=90");
  });

  it("ignores a stale response after filters change", async () => {
    const { demoDashboard } = await import("@/lib/demo-data");
    let resolveFirst!: (value: ReturnType<typeof demoDashboard>) => void;
    loadDashboardMock
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
      .mockResolvedValueOnce(demoDashboard("rent"));
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    await screen.findAllByText("Mediana mesečne najemnine");
    await act(async () => resolveFirst(demoDashboard("sale")));
    expect(screen.getAllByText("Mediana mesečne najemnine").length).toBeGreaterThan(0);
  });

  it("offers rooms only for rentals and never rental land", () => {
    render(<MarketDashboard />);
    const property = screen.getByRole("combobox", { name: "Vrsta nepremičnine" });
    expect(screen.queryByRole("option", { name: "Sobe" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    expect(screen.getByRole("option", { name: "Sobe" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Zemljišča" })).not.toBeInTheDocument();
    fireEvent.change(property, { target: { value: "room" } });
    expect(window.location.search).toContain("propertyGroup=room");
  });

  it("provides a functional mobile navigation control", () => {
    render(<MarketDashboard />);
    const menu = screen.getByRole("button", { name: "Odpri navigacijo" });
    fireEvent.click(menu);
    expect(screen.getByRole("navigation", { name: "Glavna navigacija" })).toHaveClass("open");
    expect(menu).toHaveAttribute("aria-expanded", "true");
  });

  it("returns statistics to the transaction municipality when reopening it", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Izberi Podravsko" }));
    await screen.findAllByRole("heading", { name: "Podravska" });
    fireEvent.click(screen.getByRole("button", { name: "Odpri posel" }));
    expect(window.location.search).toContain("context=transaction");
    await waitFor(() => expect(window.location.search).toContain("geographyId=77"));
    expect(screen.getAllByRole("heading", { name: "Ruše" }).length).toBeGreaterThan(0);
    await waitFor(() => expect(Element.prototype.scrollIntoView).toHaveBeenCalled());
    expect(loadDashboardMock.mock.calls.at(-1)?.[2]).toBe(77);

    fireEvent.click(screen.getByRole("button", { name: "Izberi Podravsko" }));
    await waitFor(() => expect(window.location.search).toContain("geographyId=12"));
    fireEvent.click(screen.getByRole("button", { name: "Odpri posel" }));
    await waitFor(() => expect(window.location.search).toContain("geographyId=77"));
  });
});
