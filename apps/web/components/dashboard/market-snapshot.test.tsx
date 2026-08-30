import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MarketSnapshot } from "./market-snapshot";

describe("MarketSnapshot", () => {
  it("uses explicit median and preserves sample evidence", () => {
    render(<MarketSnapshot title="Slovenija" mode="sale" propertyGroup="apartment"
      years={{ from: 2018, to: 2025 }} series={[]}
      overview={{ transactionCount: 100, eligibleCount: 90, includedSampleSize: 80,
        excludedOutlierCount: 10, medianPricePerSqm: 2500, medianPrice: 180000, medianArea: 65 }}/>)
    expect(screen.getByText("Mediana cene / m²")).toBeInTheDocument();
    expect(screen.getByText(/80 \/ 80 %/)).toBeInTheDocument();
    expect(screen.getByText(/10 izločenih statističnih odstopanj/)).toBeInTheDocument();
    expect(screen.getByText("Analizirano obdobje: 2018–2025")).toBeInTheDocument();
    expect(screen.getByText("Stanovanja · prodaja")).toBeInTheDocument();
  });
});
