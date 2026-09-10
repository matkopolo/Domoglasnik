import { loadTransactionExtremes } from "./api";
import type { MarketConstraints, MarketMode, PropertyGroup, TransactionExtreme, YearRange } from "./types";

export interface RecentTransactionsResult {
  items: TransactionExtreme[];
  from: string;
  to: string;
  expanded: boolean;
}

/** Bounded expansion, always inside the user's selected years and never into the future. */
export function recentPeriods(years: YearRange, now = new Date()) {
  const first = `${years.from}-01-01`;
  const last = `${years.to}-12-31`;
  const today = now.toISOString().slice(0, 10);
  const to = last < today ? last : today;
  if (first > to) return [];
  const end = new Date(`${to}T00:00:00Z`);
  const starts = [3, 6, 12, 24].map((months) => {
    const start = new Date(Date.UTC(end.getUTCFullYear(), end.getUTCMonth() - months + 1, 1)).toISOString().slice(0, 10);
    return start < first ? first : start;
  });
  return [...new Set([...starts, first])].map((from) => ({ from, to }));
}

export async function loadRecentTransactions(
  mode: MarketMode, group: PropertyGroup, geographyId: number | undefined,
  years: YearRange, signal: AbortSignal, area: MarketConstraints = {},
): Promise<RecentTransactionsResult> {
  const periods = recentPeriods(years);
  let result: RecentTransactionsResult = { items: [], from: `${years.from}-01-01`, to: `${years.to}-12-31`, expanded: false };
  for (const [index, period] of periods.entries()) {
    signal.throwIfAborted();
    const response = await loadTransactionExtremes(mode, group, geographyId, years, signal, area, period);
    signal.throwIfAborted();
    result = { items: response.items, ...period, expanded: index > 0 };
    if (new Set(response.items.map((item) => item.transactionId)).size >= 10) break;
  }
  return result;
}
