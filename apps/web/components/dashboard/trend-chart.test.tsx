import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TrendChart } from "./trend-chart";

afterEach(cleanup);

describe("TrendChart", () => {
  it("shows an accessible value tooltip for a yearly point", () => {
    render(
      <TrendChart
        mode="sale"
        points={[
          { period: "2023-01-01", transactionCount: 10, includedSampleSize: 9, medianPricePerSqm: 2200 },
          { period: "2024-01-01", transactionCount: 12, includedSampleSize: 11, medianPricePerSqm: 2500 },
        ]}
      />,
    );

    const point = screen.getByRole("button", { name: /2024: 2[.\s]?500 evrov/i });
    fireEvent.mouseEnter(point);
    expect(screen.getByText(/2024 · 2[.\s]?500 €\/m²/i)).toBeInTheDocument();
  });
});
