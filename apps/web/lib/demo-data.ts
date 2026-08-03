import type { Cluster, DashboardData, Distribution, Geography, Overview, TimePoint } from "./types";

export const demoGeographies: Geography[] = [
  { id: 9, level: "statistical_region", code: 9, name: "Gorenjska", bounds: [13.68, 46.0, 14.57, 46.52] },
  { id: 11, level: "statistical_region", code: 11, name: "Goriška", bounds: [13.38, 45.76, 14.15, 46.45] },
  { id: 7, level: "statistical_region", code: 7, name: "Jugovzhodna Slovenija", bounds: [14.54, 45.42, 15.41, 46.02] },
  { id: 3, level: "statistical_region", code: 3, name: "Koroška", bounds: [14.67, 46.39, 15.45, 46.66] },
  { id: 12, level: "statistical_region", code: 12, name: "Obalno-kraška", bounds: [13.56, 45.43, 14.13, 45.86] },
  { id: 8, level: "statistical_region", code: 8, name: "Osrednjeslovenska", bounds: [14.1, 45.73, 14.95, 46.37] },
  { id: 2, level: "statistical_region", code: 2, name: "Podravska", bounds: [15.31, 46.25, 16.31, 46.71] },
  { id: 1, level: "statistical_region", code: 1, name: "Pomurska", bounds: [15.79, 46.47, 16.6, 46.88] },
  { id: 6, level: "statistical_region", code: 6, name: "Posavska", bounds: [15.07, 45.79, 15.73, 46.09] },
  { id: 10, level: "statistical_region", code: 10, name: "Primorsko-notranjska", bounds: [14.04, 45.47, 14.58, 45.89] },
  { id: 4, level: "statistical_region", code: 4, name: "Savinjska", bounds: [14.57, 46.0, 15.8, 46.48] },
  { id: 5, level: "statistical_region", code: 5, name: "Zasavska", bounds: [14.71, 45.96, 15.19, 46.21] },
];

const saleOverview: Overview = {
  transactionCount: 6538,
  eligibleCount: 6306,
  includedSampleSize: 6042,
  excludedOutlierCount: 264,
  medianPrice: 155000,
  medianPricePerSqm: 2595.571,
  cleanedAveragePrice: 178915.32,
};

const saleSeries: TimePoint[] = [
  [2018, 1363.64, 10141, 9231],
  [2019, 1448.28, 10481, 9440],
  [2020, 1544.27, 8510, 7816],
  [2021, 1823.77, 10218, 9459],
  [2022, 2052.63, 9193, 8554],
  [2023, 2354.84, 8001, 7429],
  [2024, 2595.57, 6538, 6042],
  [2025, 2651.52, 7594, 6989],
].map(([year, value, count, sample]) => ({
  period: `${year}-01-01`,
  medianPricePerSqm: value,
  transactionCount: count,
  includedSampleSize: sample,
}));

const saleDistribution: Distribution = {
  metric: "pricePerSqm",
  sampleSize: 6042,
  isPublishable: true,
  percentiles: { p05: 884.95, p10: 1240.6, p25: 1836.63, p50: 2595.57, p75: 3679.58, p90: 4544.86, p95: 5000 },
  histogramLowerBound: 403.47,
  histogramUpperBound: 5955.25,
  belowBoundCount: 61,
  aboveBoundCount: 61,
  buckets: [284, 413, 731, 936, 864, 642, 622, 545, 435, 284, 162, 124].map((count, index) => ({
    index: index + 1,
    from: 403.47 + index * 462.65,
    to: 403.47 + (index + 1) * 462.65,
    count,
  })),
};

const clusterCounts = [152, 86, 69, 140, 312, 58, 176, 64, 72, 91, 47, 83];
const saleClusters: Cluster[] = demoGeographies.map((region, index) => {
  const bounds = region.bounds!;
  return {
    longitude: (bounds[0] + bounds[2]) / 2,
    latitude: (bounds[1] + bounds[3]) / 2,
    transactionCount: clusterCounts[index],
    medianAmountPerSqm: 1840 + index * 115,
  };
});

const rentOverview: Overview = {
  transactionCount: 11870,
  eligibleCount: 10142,
  includedSampleSize: 9368,
  excludedOutlierCount: 774,
  medianMonthlyRent: 650,
  medianMonthlyRentPerSqm: 11.9,
};

const rentSeries: TimePoint[] = [2018, 2019, 2020, 2021, 2022, 2023, 2024, 2025].map((year, index) => ({
  period: `${year}-01-01`,
  medianMonthlyRentPerSqm: [8.2, 8.7, 9.1, 9.4, 10.1, 10.9, 11.9, 12.6][index],
  medianMonthlyRent: [470, 490, 520, 540, 570, 600, 650, 690][index],
  transactionCount: 8200 + index * 460,
  includedSampleSize: 7100 + index * 330,
}));

const rentDistribution: Distribution = {
  ...saleDistribution,
  metric: "monthlyRent",
  sampleSize: 9368,
  percentiles: { p05: 5.1, p10: 6.2, p25: 8.4, p50: 11.9, p75: 15.3, p90: 19.8, p95: 23.5 },
  histogramLowerBound: 3.2,
  histogramUpperBound: 26.4,
};

export function demoDashboard(mode: "sale" | "rent"): DashboardData {
  return {
    overview: mode === "sale" ? saleOverview : rentOverview,
    timeSeries: mode === "sale" ? saleSeries : rentSeries,
    distribution: mode === "sale" ? saleDistribution : rentDistribution,
    clusters: saleClusters,
    geographies: demoGeographies,
    source: "live",
  };
}
