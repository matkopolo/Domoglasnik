import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ResaleSummary } from "./resale-summary";
import { loadResaleSummary } from "@/lib/property-history";
vi.mock("@/lib/property-history", () => ({ loadResaleSummary: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
it("loads only on demand, keeps filters, and never displays suppressed percentiles", async () => {
  vi.mocked(loadResaleSummary).mockResolvedValue({ pairCount: 17, assetCount: 15, isPublishable: false, medianDays: null, p25Days: null, p75Days: null });
  render(<ResaleSummary geographyId={77} propertyGroup="land" years={{ from: 2020, to: 2026 }} area={{ min: 500, landScope: "agricultural" }}/>);
  expect(loadResaleSummary).not.toHaveBeenCalled();
  fireEvent.click(screen.getByText("Čas med ponovnimi prodajami"));
  await screen.findByText(/17 parov/);
  expect(screen.getByText(/najmanj 30 parov/)).toBeInTheDocument();
  expect(screen.queryByText("Mediana")).not.toBeInTheDocument();
  await waitFor(() => expect(loadResaleSummary).toHaveBeenCalledWith("land", { from: 2020, to: 2026 }, 77, { min: 500, landScope: "agricultural" }, expect.any(AbortSignal)));
});
