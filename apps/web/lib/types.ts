export type MarketMode = "sale" | "rent";
export type PropertyGroup = "apartment" | "house" | "land" | "commercial" | "garage" | "room";

export interface Geography {
  id: number;
  level: string;
  code?: number;
  name: string;
  parentId?: number;
  bounds?: [number, number, number, number];
}

export interface Overview {
  transactionCount: number;
  eligibleCount: number;
  includedSampleSize: number;
  excludedOutlierCount: number;
  medianPrice?: number;
  medianPricePerSqm?: number;
  cleanedAveragePrice?: number;
  medianMonthlyRent?: number;
  medianMonthlyRentPerSqm?: number;
}

export interface TimePoint {
  period: string;
  transactionCount: number;
  includedSampleSize: number;
  medianPricePerSqm?: number;
  medianMonthlyRentPerSqm?: number;
  medianPrice?: number;
  medianMonthlyRent?: number;
  yearOverYearMedianRentPerSqmPercent?: number | null;
}

export interface Distribution {
  metric: string;
  sampleSize: number;
  percentiles: Record<"p05" | "p10" | "p25" | "p50" | "p75" | "p90" | "p95", number>;
  histogramLowerBound: number;
  histogramUpperBound: number;
  belowBoundCount: number;
  aboveBoundCount: number;
  buckets: Array<{ index: number; from: number; to: number; count: number }>;
}

export interface Cluster {
  longitude: number;
  latitude: number;
  transactionCount: number;
  medianAmountPerSqm?: number;
}

export interface DashboardData {
  overview: Overview;
  timeSeries: TimePoint[];
  distribution: Distribution;
  clusters: Cluster[];
  geographies: Geography[];
  source: "live" | "demo";
}
