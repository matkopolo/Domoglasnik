import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ContextPanel } from "./context-panel";

vi.mock("@/lib/api", () => ({
  loadTransaction: vi.fn(async () => ({
    transactionId: 77,
    contractDate: "2024-05-03",
    datasetYear: 2024,
    propertyGroup: "apartment",
    area: 50,
    monthlyRentExVat: 700,
    monthlyRentPerSqm: 14,
    rentDerivation: "reported",
    rentalTerm: "fixed",
    isMarketRent: true,
    quality: { isQualityEligible: true, isRepresentative: true, reasons: [] },
    geographies: [{ id: 2, level: "statistical_region", name: "Podravska" }],
    spaces: [{
      id: 1,
      rentedSpaceTypeDescription: "Stanovanje",
      constructionYear: 2001,
      street: "Hmeljarska ulica",
      houseNumber: "8",
      settlement: "Ruše",
    }],
  })),
  loadTransactionContext: vi.fn(async () => ({
    transactionKind: "rent",
    transactionId: 77,
    contractYear: 2024,
    incompletePeriod: false,
    propertyGroup: "apartment",
    actualAmountPerSqm: 14,
    benchmarks: [{
      scope: "municipality",
      label: "Ruše",
      medianAmountPerSqm: 12,
      sampleSize: 31,
      differencePercent: 16.67,
      percentile: 70,
      period: 2024,
      reliability: "moderate",
      isPublishable: true,
    }],
    quality: { isQualityEligible: true, isRepresentative: true, reasons: [] },
    methodologyVersion: "test",
  })),
  loadBuildingStatistics: vi.fn(),
  loadGeography: vi.fn(),
  loadGeographyChildren: vi.fn(),
  loadMarketIndices: vi.fn(async () => ({ items: [] })),
  loadFullValuation: vi.fn(async () => {
    throw new Error("no model");
  }),
}));

describe("ContextPanel", () => {
  it("renders rental detail and backend-computed context", async () => {
    render(<ContextPanel
      context={{ kind: "transaction", mode: "rent", id: 77 }}
      mode="rent"
      propertyGroup="apartment"
      years={{ from: 2024, to: 2024 }}
      overview={{ transactionCount: 0, eligibleCount: 0, includedSampleSize: 0, excludedOutlierCount: 0 }}
      onChange={vi.fn()}
    />);
    expect(await screen.findByText("700 €")).toBeInTheDocument();
    expect(screen.getByText("14 €/m²")).toBeInTheDocument();
    expect(screen.getByText("+16,67 %")).toBeInTheDocument();
    expect(screen.getByText(/n=31/)).toBeInTheDocument();
    expect(screen.queryByText("Ni podatka")).not.toBeInTheDocument();
  });
});
