import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DistributionChart } from "./distribution-chart";

describe("DistributionChart", () => {
  it("exposes the exact transaction count and price range for every bar", () => {
    render(<DistributionChart mode="sale" data={{
      metric: "price_per_sqm",
      sampleSize: 47,
      isPublishable: true,
      belowBoundCount: 0,
      aboveBoundCount: 0,
      buckets: [
        { index: 1, from: 1000, to: 1500, count: 43 },
        { index: 2, from: 1500, to: 2000, count: 4 },
      ],
      percentiles: { p05: 1050, p10: 1100, p25: 1250, p50: 1450, p75: 1700, p90: 1900, p95: 1950 },
    }} />);

    expect(screen.getByRole("button", { name: /1000–1500 €\/m²: 43 poslov/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /1500–2000 €\/m²: 4 posli/ })).toBeInTheDocument();
    expect(screen.getByText("43 poslov")).toBeInTheDocument();
    expect(screen.getByText("Število poslov")).toBeInTheDocument();
    expect(screen.getByText("Cena na m²")).toBeInTheDocument();
  });
});
