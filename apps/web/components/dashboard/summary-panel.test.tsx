import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SummaryPanel } from "./summary-panel";

describe("SummaryPanel", () => {
  it("provides an accessible explanation for its market indicators", () => {
    render(<SummaryPanel title="Ruše" mode="sale" series={[]} overview={{
      transactionCount: 42,
      eligibleCount: 38,
      includedSampleSize: 38,
      excludedOutlierCount: 4,
      medianPricePerSqm: 2100,
      medianPrice: 140000,
    }} />);

    const help = screen.getByRole("button", { name: "Pojasnilo kazalnikov" });
    const tooltip = screen.getByRole("tooltip");
    expect(help).toHaveAttribute("aria-describedby", tooltip.id);
    expect(tooltip).toHaveTextContent("Statistična odstopanja niso vključena");
  });
});
