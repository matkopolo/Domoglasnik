export type MarketMode = "sale" | "rent";
export type PropertyGroup = "apartment" | "house" | "land" | "commercial" | "garage" | "room";
export interface YearRange { from: number; to: number }

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
  medianPricePerSqm?: number;
  medianMonthlyRentPerSqm?: number;
  representativeCount?: number;
}

export interface SaleTransactionDetail {
  transactionId: number;
  contractDate?: string;
  propertyGroup: string;
  area?: number;
  price?: number;
  pricePerSqm?: number;
  buildingParts: Array<{
    buildingPartType?: number;
    constructionYear?: number;
    floor?: string;
    positionInBuilding?: string;
    unitNumber?: string;
    roomCount?: number;
    soldArea?: number;
    soldUsableArea?: number;
    buildingPartArea?: number;
    usableArea?: number;
    outdoorParkingSpaces?: number;
    settlement?: string;
    street?: string;
    houseNumber?: string;
  }>;
  landParcels: Array<{ parcelArea?: number }>;
}

export interface BuildingSearchResult {
  cadastralMunicipalityCode: number;
  buildingNumber: string;
  settlement?: string;
  street?: string;
  houseNumber?: string;
  houseNumberSuffix?: string;
  longitude: number;
  latitude: number;
  saleCount: number;
}

export interface BuildingStatistics {
  cadastralMunicipalityCode: number;
  buildingNumber: string;
  constructionYear?: number;
  knownBuildingPartCount: number;
  longitude?: number;
  latitude?: number;
  sales: { transactionCount: number; medianAmount?: number; medianAmountPerSqm?: number };
  saleTransactions: Array<{
    transactionId: number;
    contractDate?: string;
    propertyGroup: string;
    area?: number;
    amount?: number;
    amountPerSqm?: number;
  }>;
}

export type MapFeatureProperties = Record<string, string | number | boolean | null | number[]>;
export type MapPointFeature = Feature<Point, MapFeatureProperties>;
export type MapFeatureCollection = FeatureCollection<Point, MapFeatureProperties>;
export type BoundaryProperties = Record<string, string | number | boolean | null | number[]>;
export type BoundaryFeature = Feature<Polygon | MultiPolygon, BoundaryProperties>;
export type BoundaryFeatureCollection = FeatureCollection<Polygon | MultiPolygon, BoundaryProperties>;

export interface DashboardData {
  overview: Overview;
  timeSeries: TimePoint[];
  distribution: Distribution;
  clusters: Cluster[];
  geographies: Geography[];
  source: "live" | "demo";
}
import type { Feature, FeatureCollection, MultiPolygon, Point, Polygon } from "geojson";
