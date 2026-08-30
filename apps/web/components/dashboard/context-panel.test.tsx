import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
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
    quality: { isQualityEligible: true, isRepresentative: true, reasons: [], anomalyFlags: ["possible_near_duplicate"] },
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
    outlierStatus: "insufficient_cohort",
    directComparables: [{
      transactionId: 78,
      contractDate: "2024-08-10",
      contractYear: 2024,
      area: 52,
      amount: 750,
      amountPerSqm: 14.42,
      areaDifferencePercent: 4,
      amountPerSqmDifferencePercent: 3,
      yearDifference: 0,
      matchTier: "same_building_same_year_area_15",
      isRepresentative: true,
    }],
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
      comparisonProfile: "unadjusted",
    }],
    currentMarketEstimate: {
      lowerTotal: 150000,
      centralTotal: 157000,
      upperTotal: 165000,
      unadjustedLowerTotal: 160000,
      unadjustedCentralTotal: 172500,
      unadjustedUpperTotal: 181000,
      lowerAmountPerSqm: 2885,
      centralAmountPerSqm: 3019,
      upperAmountPerSqm: 3173,
      unadjustedLowerAmountPerSqm: 3077,
      unadjustedCentralAmountPerSqm: 3317,
      unadjustedUpperAmountPerSqm: 3481,
      comparableCount: 60,
      maximumRadiusMeters: 20000,
      areaTolerancePercent: 15,
      constructionYearTolerance: 8,
      locationAdjustmentFactor: 0.91,
      rawLocationAdjustmentFactor: 0.78,
      locationAdjustmentApplied: true,
      localAdjustmentSampleSize: 7,
      packageCompositionMatched: true,
      referenceYear: 2026,
      confidence: "low",
      warnings: ["statistična tržna ocena, ne uradna cenitev"],
      methodologyVersion: "test",
    },
    quality: { isQualityEligible: true, isRepresentative: true, reasons: [], anomalyFlags: ["possible_near_duplicate"] },
    methodologyVersion: "test",
  })),
  loadBuildingStatistics: vi.fn(async () => ({
    cadastralMunicipalityCode: 665,
    buildingNumber: "8",
    constructionYear: 2001,
    knownBuildingPartCount: 4,
    sales: { transactionCount: 1, representativeCount: 1 },
    rentals: { transactionCount: 0, representativeCount: 0 },
    saleHistory: [],
    rentalHistory: [],
    saleTransactions: [{
      transactionId: 501,
      contractDate: "2025-06-12",
      propertyGroup: "apartment",
      area: 72,
      amount: 194000,
      amountPerSqm: 2694.44,
    }],
    rentalTransactions: [],
  })),
  loadGeography: vi.fn(),
  loadGeographyChildren: vi.fn(),
  loadMarketIndices: vi.fn(async () => ({ items: [] })),
  loadFullValuation: vi.fn(async () => {
    throw new Error("no model");
  }),
}));

describe("ContextPanel", () => {
  afterEach(cleanup);
  it("renders rental detail and backend-computed context", async () => {
    const onChange = vi.fn();
    render(<ContextPanel
      context={{ kind: "transaction", mode: "rent", id: 77 }}
      mode="rent"
      propertyGroup="apartment"
      years={{ from: 2024, to: 2024 }}
      overview={{ transactionCount: 0, eligibleCount: 0, includedSampleSize: 0, excludedOutlierCount: 0 }}
      onChange={onChange}
    />);
    expect(await screen.findByText("700 €")).toBeInTheDocument();
    expect(screen.getByText("Možen podvojen zapis")).toBeInTheDocument();
    expect(screen.getByText("14 €/m²")).toBeInTheDocument();
    expect(screen.getByText("157.000 €")).toBeInTheDocument();
    expect(screen.getAllByText("primerljivih poslov", { exact: false }).length).toBeGreaterThan(0);
    expect(screen.getByText(/P25–P75 je srednjih 50 %/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Primerljivi posli" }));
    expect(screen.getByText(/\+16,67 % nad mediano/)).toBeInTheDocument();
    expect(screen.getByText(/n=31/)).toBeInTheDocument();
    expect(screen.getByText(/širši tržni vzorec/)).toBeInTheDocument();
    expect(screen.getByText("750 €")).toBeInTheDocument();
    expect(screen.getByText(/Ista stavba · isto leto/)).toBeInTheDocument();
    expect(screen.getByText("Primerjava z okolico")).toBeInTheDocument();
    fireEvent.click(screen.getByText("750 €"));
    expect(onChange).toHaveBeenCalledWith({ kind: "transaction", mode: "rent", id: 78 });
    fireEvent.click(screen.getByRole("tab", { name: "Kakovost in model" }));
    expect(screen.getByText("Premalo primerljivih poslov za oceno odstopanja")).toBeInTheDocument();
    expect(screen.queryByText(/insufficient_cohort/)).not.toBeInTheDocument();
    expect(screen.queryByText("Ni podatka")).not.toBeInTheDocument();
  });

  it("supports keyboard navigation between inspector tabs", async () => {
    render(<ContextPanel context={{ kind: "transaction", mode: "rent", id: 77 }} mode="rent"
      propertyGroup="apartment" years={{ from: 2024, to: 2024 }}
      overview={{ transactionCount: 0, eligibleCount: 0, includedSampleSize: 0, excludedOutlierCount: 0 }} onChange={vi.fn()}/>);
    const first = await screen.findByRole("tab", { name: "Pregled" });
    fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(screen.getByRole("tab", { name: "Primerljivi posli" })).toHaveAttribute("aria-selected", "true");
  });

  it("presents building sales as actionable evidence with key values", async () => {
    const onChange = vi.fn();
    render(<ContextPanel context={{ kind: "building", cadastralCode: 665, buildingNumber: "8" }} mode="sale"
      propertyGroup="apartment" years={{ from: 2024, to: 2025 }}
      overview={{ transactionCount: 0, eligibleCount: 0, includedSampleSize: 0, excludedOutlierCount: 0 }} onChange={onChange}/>);
    const sale = await screen.findByRole("button", { name: "Odpri analizo prodaje #501" });
    expect(sale).toHaveTextContent("194.000 €");
    expect(sale).toHaveTextContent("2694,44 €/m²");
    expect(sale).toHaveTextContent("72 m²");
    expect(sale).toHaveTextContent("Odpri analizo posla");
    fireEvent.click(sale);
    expect(onChange).toHaveBeenCalledWith({ kind: "transaction", mode: "sale", id: 501 });
  });
});
