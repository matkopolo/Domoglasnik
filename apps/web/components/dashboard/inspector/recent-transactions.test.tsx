import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { RecentTransactions } from "./recent-transactions";
import { loadRecentTransactions, type RecentTransactionsResult } from "@/lib/recent-transactions";

vi.mock("@/lib/recent-transactions", () => ({ loadRecentTransactions: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
const base = { mode: "sale" as const, propertyGroup: "apartment" as const, geographyId: 77, name: "Ruše", years: { from: 2025, to: 2025 }, area: { min: 60 }, onChange: vi.fn() };
const result: RecentTransactionsResult = {
  from: "2025-07-01", to: "2025-12-31", expanded: true,
  items: [{ transactionId: 123, direction: "highest", propertyGroup: "apartment", amount: 180000, amountPerSqm: 2500, area: 72, contractDate: "2025-10-20" }],
};

it("shows source amounts, area, date, actual search period and opens the transaction", async () => {
  vi.mocked(loadRecentTransactions).mockResolvedValue(result);
  render(<RecentTransactions {...base}/>);
  const transaction = await screen.findByRole("button", { name: "Odpri prodajo #123" });
  expect(transaction).toHaveTextContent("72 m²");
  expect(transaction).toHaveTextContent("2500 €/m²");
  expect(transaction).toHaveTextContent("180.000 €");
  expect(transaction).toHaveTextContent("20. 10. 2025");
  expect(screen.getByText(/obdobje razširjeno/)).toBeInTheDocument();
  fireEvent.click(transaction);
  expect(base.onChange).toHaveBeenCalledWith({ kind: "transaction", mode: "sale", id: 123 });
});

it("ignores results from a previous area even if the loader ignores cancellation", async () => {
  let resolveOld!: (value: RecentTransactionsResult) => void;
  vi.mocked(loadRecentTransactions).mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve; }))
    .mockResolvedValueOnce({ ...result, items: [] });
  const view = render(<RecentTransactions {...base}/>);
  view.rerender(<RecentTransactions {...base} geographyId={78} name="Maribor"/>);
  await screen.findByText(/ni ustreznih reprezentativnih poslov/);
  await act(async () => resolveOld(result));
  expect(screen.queryByRole("button", { name: "Odpri prodajo #123" })).not.toBeInTheDocument();
});

it("distinguishes an API failure from an empty population", async () => {
  vi.mocked(loadRecentTransactions).mockRejectedValue(new Error("API unavailable"));
  render(<RecentTransactions {...base}/>);
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("ni bilo mogoče naložiti"));
  expect(screen.queryByText(/ni ustreznih reprezentativnih poslov/)).not.toBeInTheDocument();
});
