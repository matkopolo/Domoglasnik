import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PropertyHistory } from "./property-history";
import { LocationLink } from "./location-link";
import { loadPropertyHistory, type PropertyHistory as History } from "@/lib/property-history";

vi.mock("@/lib/property-history", async (original) => ({ ...await original<typeof import("@/lib/property-history")>(), loadPropertyHistory: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
const history: History = { transactionId: 1, truncated: false, assets: [{ kind: "parcel", cadastralCode: 678, number: "5/1", partNumber: "", truncated: false, pairs: [], events: [
  { transactionId: 123, componentId: 1, contractDate: "2025-01-01", evidence: "exact_identity", propertyGroup: "land", sourceType: 7, share: "1/2", area: 500, agreementPrice: 100000, componentPrice: null, constructionYear: null, componentCount: 2, representative: false, possibleDuplicate: true },
] }] };
it("shows source identity, share, package warning and navigates to recorded sales", async () => {
  vi.mocked(loadPropertyHistory).mockResolvedValue(history);
  const change = vi.fn(); render(<PropertyHistory transactionId={1} onChange={change}/>);
  fireEvent.click(await screen.findByRole("button", { name: /Posel #123/ }));
  expect(change).toHaveBeenCalledWith({ kind: "transaction", mode: "sale", id: 123 });
  expect(screen.getByText("1/2")).toBeInTheDocument();
  expect(screen.getByText(/Skupna cena ni cena samo/)).toBeInTheDocument();
  expect(screen.getByText(/Možen podvojen zapis/)).toBeInTheDocument();
});
it("does not accept stale results after selecting another transaction", async () => {
  let resolve!: (data: History) => void;
  vi.mocked(loadPropertyHistory).mockReturnValueOnce(new Promise((r) => { resolve = r; })).mockResolvedValueOnce({ transactionId: 2, assets: [], truncated: false });
  const view = render(<PropertyHistory transactionId={1} onChange={vi.fn()}/>);
  view.rerender(<PropertyHistory transactionId={2} onChange={vi.fn()}/>);
  await screen.findByText(/ni dovolj popolnih katastrskih/);
  await act(async () => resolve(history));
  expect(screen.queryByRole("button", { name: /Posel #123/ })).not.toBeInTheDocument();
});
it("keeps an error distinct from missing identities", async () => {
  vi.mocked(loadPropertyHistory).mockRejectedValue(new Error("offline"));
  render(<PropertyHistory transactionId={1} onChange={vi.fn()}/>);
  expect(await screen.findByRole("alert")).toHaveTextContent("Zgodovine ni bilo mogoče naložiti");
});
it("opens Google externally without a misleading missing-coordinate link", () => {
  const view = render(<LocationLink latitude={46} longitude={15}/>);
  expect(screen.getByRole("link")).toHaveAttribute("rel", "noopener noreferrer");
  expect(screen.getByText(/Približna lokacija ETN/)).toBeInTheDocument();
  view.rerender(<LocationLink latitude={null} longitude={15}/>);
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
});
