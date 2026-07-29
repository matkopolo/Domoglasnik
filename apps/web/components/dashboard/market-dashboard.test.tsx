import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MarketDashboard } from "./market-dashboard";

const { loadDashboardMock } = vi.hoisted(() => ({ loadDashboardMock: vi.fn() }));
vi.mock("@/lib/api", async () => {
  return {
    loadDashboard: loadDashboardMock,
    loadMarketIndices: vi.fn(async () => ({ items: [] })),
    loadDetailedTimeSeries: vi.fn(async () => ({ items: [] })),
    loadStatisticalOutliers: vi.fn(async () => ({ items: [], total: 0 })),
    loadAnomalySummary: vi.fn(async () => ({ total: 0, flagged: 0, unreviewed: 0, highPriority: 0 })),
    loadTransactionExtremes: vi.fn(async () => ({ items: [], population: "representative" })),
    loadGeographyRankings: vi.fn(async () => []),
  };
});

vi.mock("./slovenia-map", () => ({
  SloveniaMap: () => <div role="application" aria-label="Interaktivni zemljevid nepremičninskih poslov v Sloveniji" />,
}));

afterEach(cleanup);

describe("MarketDashboard", () => {
  beforeEach(async () => {
    const { demoDashboard } = await import("@/lib/demo-data");
    loadDashboardMock.mockReset();
    loadDashboardMock.mockImplementation(async (mode: "sale" | "rent") => demoDashboard(mode));
    window.history.replaceState(null, "", "/");
  });

  it("renders the map-first market overview", async () => {
    render(<MarketDashboard />);
    expect(screen.getByRole("heading", { name: "Analiza nepremičninskega trga" })).toBeInTheDocument();
    expect(screen.getByRole("application", { name: "Interaktivni zemljevid nepremičninskih poslov v Sloveniji" })).toBeInTheDocument();
    const metric = await screen.findByText("Srednja cena / m²");
    expect(metric.nextElementSibling).toHaveTextContent("2596");
  });

  it("switches to rental metrics", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    const metric = await screen.findByText("Srednja najemnina / m²");
    expect(metric.nextElementSibling).toHaveTextContent("12");
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

  it("ignores a stale response after filters change", async () => {
    const { demoDashboard } = await import("@/lib/demo-data");
    let resolveFirst!: (value: ReturnType<typeof demoDashboard>) => void;
    loadDashboardMock
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
      .mockResolvedValueOnce(demoDashboard("rent"));
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    await screen.findByText("Srednja najemnina / m²");
    await act(async () => resolveFirst(demoDashboard("sale")));
    expect(screen.getByText("Srednja najemnina / m²")).toBeInTheDocument();
  });
});
