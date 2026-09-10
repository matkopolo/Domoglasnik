import { afterEach, describe, expect, it, vi } from "vitest";
import { loadRecentTransactions, recentPeriods } from "./recent-transactions";

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });

describe("recent price rankings", () => {
  it("uses bounded calendar windows inside the selected years", () => {
    expect(recentPeriods({ from: 2025, to: 2025 }, new Date("2026-09-07"))).toEqual([
      { from: "2025-10-01", to: "2025-12-31" },
      { from: "2025-07-01", to: "2025-12-31" },
      { from: "2025-01-01", to: "2025-12-31" },
    ]);
    expect(recentPeriods({ from: 2026, to: 2027 }, new Date("2026-09-07"))[0]).toEqual({ from: "2026-07-01", to: "2026-09-07" });
    expect(recentPeriods({ from: 2027, to: 2027 }, new Date("2026-09-07"))).toEqual([]);
  });

  it("expands sparse results, retains every filter, and stops at ten distinct transactions", async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date("2026-09-07"));
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ items: [] })))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: Array.from({ length: 10 }, (_, index) => ({ transactionId: index, direction: "highest" })) })));
    vi.stubGlobal("fetch", fetchMock);
    const result = await loadRecentTransactions("sale", "land", 77, { from: 2025, to: 2025 }, new AbortController().signal, { min: 60, max: 90, landScope: "all" });
    expect(result.expanded).toBe(true);
    expect(result.from).toBe("2025-07-01");
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const query = new URL(fetchMock.mock.calls[1][0]).searchParams;
    expect(Object.fromEntries(query)).toMatchObject({ limit: "10", geographyId: "77", minArea: "60", maxArea: "90", landScope: "all", transactionKind: "sale", propertyGroup: "land" });
  });

  it("returns fewer than ten when even the entire selected period is sparse", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [{ transactionId: 7, direction: "lowest" }, { transactionId: 7, direction: "highest" }] }))));
    const result = await loadRecentTransactions("rent", "apartment", 77, { from: 2025, to: 2025 }, new AbortController().signal);
    expect(result.from).toBe("2025-01-01");
    expect(result.items).toHaveLength(2);
  });

  it("does not continue expanding after cancellation", async () => {
    const controller = new AbortController();
    const fetchMock = vi.fn(async () => { controller.abort(); return new Response(JSON.stringify({ items: [] })); });
    vi.stubGlobal("fetch", fetchMock);
    await expect(loadRecentTransactions("sale", "apartment", 77, { from: 2025, to: 2025 }, controller.signal)).rejects.toThrow();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
