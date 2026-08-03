import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AnalyticsWorkspace } from "./analytics-workspace";

vi.mock("@/lib/api", () => ({
  loadDetailedTimeSeries: vi.fn(async () => ({ items: [{
    period: "2025-01-01",
    transactionCount: 100,
    includedSampleSize: 80,
    excludedOutlierCount: 5,
    medianPricePerSqm: 2500,
  }] })),
  loadMarketIndices: vi.fn(async () => ({ items: [{
    transactionKind: "sale",
    method: "hedonic",
    propertyGroup: "apartment",
    period: "2025-01-01",
    indexValue: 121,
    sampleSize: 80,
    basePeriod: "2018-01-01",
    isPublishable: true,
  }, {
    transactionKind: "sale",
    method: "matched_pair_median",
    propertyGroup: "apartment",
    period: "2025-01-01",
    indexValue: 118,
    sampleSize: 41,
    basePeriod: "2018-01-01",
    isPublishable: true,
  }] })),
  loadStatisticalOutliers: vi.fn(async () => ({ items: [{
    transactionId: 99,
    propertyGroup: "apartment",
    pricePerSqm: 9000,
    cohortLevel: "municipality_year",
    cohortSize: 40,
    cohortMedian: 2000,
    lowerBound: 1000,
    upperBound: 4000,
    modifiedZScore: 7.2,
    ruleVersion: "test",
  }], total: 1 })),
  loadAnomalySummary: vi.fn(async () => ({
    total: 100, flagged: 20, unreviewed: 15, highPriority: 4,
  })),
  loadTransactionExtremes: vi.fn(async () => ({ population: "representative", items: [{
    direction: "highest",
    transactionId: 501,
    propertyGroup: "apartment",
    amount: 400000,
    area: 80,
    amountPerSqm: 5000,
  }, {
    direction: "lowest",
    transactionId: 502,
    propertyGroup: "apartment",
    amount: 80000,
    area: 80,
    amountPerSqm: 1000,
  }] })),
  loadGeographyRankings: vi.fn(async () => [{
    id: 2,
    name: "Podravska",
    level: "statistical_region",
    transactionCount: 100,
    representativeCount: 80,
    medianAmountPerSqm: 2200,
  }]),
  loadGeography: vi.fn(),
  loadGeographyChildren: vi.fn(),
}));

describe("AnalyticsWorkspace", () => {
  it("exposes indices, geography rankings, and outlier diagnostics", async () => {
    const onContextChange = vi.fn();
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    const mapPanel = document.createElement("section");
    mapPanel.className = "map-panel";
    document.body.appendChild(mapPanel);
    render(<AnalyticsWorkspace
      mode="sale"
      propertyGroup="apartment"
      years={{ from: 2018, to: 2025 }}
      context={{ kind: "country" }}
      overview={{
        transactionCount: 100,
        eligibleCount: 90,
        includedSampleSize: 80,
        excludedOutlierCount: 10,
        medianPrice: 180000,
        medianArea: 65,
        isPublishable: true,
      }}
      onContextChange={onContextChange}
    />);
    expect(await screen.findByText("180.000 €")).toBeInTheDocument();
    expect(screen.getByText("Vsi evidentirani posli")).toBeInTheDocument();
    expect(screen.getByText("#501")).toBeInTheDocument();
    expect(screen.getByText("#502")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Indeksi in modeli/ }));
    expect((await screen.findAllByText("Približno 21 % višje kot leta 2018")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("Približno 18 % višje kot leta 2018").length).toBeGreaterThan(0);
    expect(screen.getByText(/indeks 121/)).toBeInTheDocument();
    expect(screen.getByText("Indeks ponovnih prodaj")).toBeInTheDocument();
    expect(screen.getByText("Raven cen (začetno leto = 100)")).toBeInTheDocument();
    expect(screen.getByText("Leto")).toBeInTheDocument();
    const hedonicPoint = screen.getByRole("button", { name: /Hedonski indeks, 2025-01: indeks 121/ });
    fireEvent.mouseEnter(hedonicPoint);
    expect(screen.getByText(/Hedonski indeks · 2025-01 · indeks 121/)).toBeInTheDocument();
    expect(screen.getByText(/Približno 21 % višje kot leta 2018 · n=80/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Primerjava območij/ }));
    expect(screen.getByText("Podravska")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Nazaj na zemljevid/ })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Podravska/ }));
    expect(onContextChange).toHaveBeenCalledWith({
      kind: "geography",
      id: 2,
      level: "statistical_region",
      name: "Podravska",
    });
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: /Odstopanja/ }));
    expect(screen.getByText("#99")).toBeInTheDocument();
    expect(screen.getByText("7,2")).toBeInTheDocument();
  });
});
