import type { Feature, FeatureCollection, MultiPolygon, Point, Polygon } from "geojson";
import type { components } from "@/lib/generated/api-schema";

type ApiSchemas = components["schemas"];

export type MarketMode = ApiSchemas["Metadata"]["transactionKinds"][number];
export type PropertyGroup = Exclude<ApiSchemas["PropertyGroup"], "mixed" | "unknown">;
export type GeographyLevel = ApiSchemas["GeographyLevel"];
export interface YearRange { from: number; to: number }
export interface AreaRange { min?: number; max?: number }

export interface Geography {
  id: number;
  level: GeographyLevel;
  code?: number;
  name: string;
  parentId?: number;
  bounds?: [number, number, number, number];
}

export interface GeographyComparison {
  id: number;
  name: string;
  level: GeographyLevel;
  transactionCount: number;
  representativeCount: number;
  medianAmountPerSqm?: number | null;
}

export interface TransactionExtreme {
  direction: "lowest" | "highest";
  transactionId: number;
  contractDate?: string | null;
  propertyGroup: PropertyGroup;
  area?: number | null;
  amount?: number | null;
  amountPerSqm: number;
}

export interface Overview {
  transactionCount: number;
  eligibleCount: number;
  includedSampleSize: number;
  excludedOutlierCount: number;
  medianPrice?: number | null;
  medianPricePerSqm?: number | null;
  cleanedAveragePrice?: number | null;
  medianMonthlyRent?: number | null;
  medianMonthlyRentPerSqm?: number | null;
  priceSampleSize?: number;
  pricePerSqmSampleSize?: number;
  rawAveragePrice?: number | null;
  rawAveragePricePerSqm?: number | null;
  cleanedAveragePricePerSqm?: number | null;
  medianArea?: number | null;
  isPublishable?: boolean;
  suppressionReason?: string | null;
}

export interface TimePoint {
  period: string;
  transactionCount: number;
  includedSampleSize: number;
  medianPricePerSqm?: number | null;
  medianMonthlyRentPerSqm?: number | null;
  medianPrice?: number | null;
  medianMonthlyRent?: number | null;
  yearOverYearMedianRentPerSqmPercent?: number | null;
  incomplete?: boolean;
  eligibleCount?: number;
  excludedOutlierCount?: number;
  rawAveragePrice?: number | null;
  rawAveragePricePerSqm?: number | null;
  cleanedAveragePrice?: number | null;
  cleanedAveragePricePerSqm?: number | null;
  medianArea?: number | null;
  isPublishable?: boolean;
  suppressionReason?: string | null;
}

export interface InflationPoint {
  period: string;
  annualRatePercent: number;
}

export interface Distribution {
  metric: string;
  sampleSize: number;
  isPublishable: boolean;
  suppressionReason?: string | null;
  percentiles?: Record<"p05" | "p10" | "p25" | "p50" | "p75" | "p90" | "p95", number> | null;
  histogramLowerBound?: number;
  histogramUpperBound?: number;
  belowBoundCount: number;
  aboveBoundCount: number;
  buckets: Array<{ index: number; from: number; to: number; count: number }>;
}

export interface Cluster {
  longitude: number;
  latitude: number;
  transactionCount: number;
  medianAmountPerSqm?: number | null;
  medianPricePerSqm?: number | null;
  medianMonthlyRentPerSqm?: number | null;
  representativeCount?: number;
}

export interface QualityDecision {
  isQualityEligible: boolean;
  isRepresentative: boolean;
  reasons: string[];
}

export interface OutlierEvidence {
  status: string;
  method?: string | null;
  cohortLevel?: string | null;
  cohortSize?: number | null;
  cohortMedian?: number | null;
  lowerBound?: number | null;
  upperBound?: number | null;
  modifiedZScore?: number | null;
  ruleVersion: string;
}

export interface TransactionGeography {
  id: number;
  level: GeographyLevel;
  name: string;
}

export interface SaleBuildingPart {
  id: number;
  cadastralMunicipalityCode?: number | null;
  buildingNumber?: string | null;
  buildingPartNumber?: string | null;
  buildingPartType?: number | null;
  buildingPartTypeDescription?: string | null;
  constructionYear?: number | null;
  floor?: string | null;
  positionInBuilding?: string | null;
  roomCount?: number | null;
  soldArea?: number | null;
  soldUsableArea?: number | null;
  buildingPartArea?: number | null;
  usableArea?: number | null;
  outdoorParkingSpaces?: number | null;
  contractualPrice?: number | null;
  settlement?: string | null;
  street?: string | null;
  houseNumber?: string | null;
}

export interface SaleLandParcel {
  id: number;
  cadastralMunicipalityCode?: number | null;
  parcelNumber?: string | null;
  landType?: number | null;
  landTypeDescription?: string | null;
  parcelArea?: number | null;
  contractualPrice?: number | null;
}

export interface SaleTransactionDetail {
  transactionId: number;
  contractDate?: string | null;
  datasetYear: number;
  propertyGroup: PropertyGroup;
  area?: number | null;
  price?: number | null;
  pricePerSqm?: number | null;
  priceDerivation: string;
  longitude?: number | null;
  latitude?: number | null;
  quality: QualityDecision;
  outlier?: OutlierEvidence | null;
  geographies: TransactionGeography[];
  buildingParts: SaleBuildingPart[];
  landParcels: SaleLandParcel[];
}

export interface RentalSpace {
  id: number;
  cadastralMunicipalityCode?: number | null;
  buildingNumber?: string | null;
  buildingPartNumber?: string | null;
  rentedSpaceType?: number | null;
  rentedSpaceTypeDescription?: string | null;
  furnishing?: number | null;
  area?: number | null;
  usableArea?: number | null;
  contractualRent?: number | null;
  constructionYear?: number | null;
  settlement?: string | null;
  street?: string | null;
  houseNumber?: string | null;
}

export interface RentalTransactionDetail {
  transactionId: number;
  contractDate?: string | null;
  rentalStartDate?: string | null;
  rentalEndDate?: string | null;
  datasetYear: number;
  propertyGroup: PropertyGroup;
  area?: number | null;
  monthlyRentReported?: number | null;
  monthlyRentExVat?: number | null;
  monthlyRentPerSqm?: number | null;
  rentDerivation: string;
  rentalTerm: string;
  durationMonths?: number | null;
  operatingCostsIncluded?: boolean | null;
  isMarketRent: boolean;
  longitude?: number | null;
  latitude?: number | null;
  quality: QualityDecision;
  outlier?: OutlierEvidence | null;
  geographies: TransactionGeography[];
  spaces: RentalSpace[];
}

export interface ContextBenchmark {
  scope: string;
  label: string;
  geographyId?: number;
  medianAmountPerSqm: number;
  sampleSize: number;
  differencePercent: number;
  percentile: number;
  period: number;
  radiusMeters?: number;
  reliability: "very_low" | "low" | "moderate";
  isPublishable: boolean;
  comparisonProfile: "area_15_age_10" | "area_25_age_15" | "area_15" | "area_25" | "unadjusted";
  areaTolerancePercent?: number;
  constructionYearTolerance?: number;
}

export interface DirectComparable {
  transactionId: number;
  contractDate: string;
  contractYear: number;
  area: number;
  amount: number;
  amountPerSqm: number;
  areaDifferencePercent: number;
  amountPerSqmDifferencePercent: number;
  yearDifference: number;
  matchTier:
    | "same_building_same_year_area_15"
    | "same_building_adjacent_year_area_15"
    | "same_building_same_year"
    | "same_building_other";
  constructionYear?: number;
  buildingPartNumber?: string;
  isRepresentative: boolean;
  outlierStatus?: string;
}

export interface ValuationPrediction {
  actualAmountPerSqm: number;
  predictedAmountPerSqm: number;
  lowerAmountPerSqm: number;
  upperAmountPerSqm: number;
  predictedTotal: number;
  lowerTotal: number;
  upperTotal: number;
  modelVersion: string;
  reliability: "very_low" | "low" | "moderate";
  warnings: string[];
}

export interface MarketIndex {
  transactionKind: MarketMode;
  method: "matched_pair_median" | "hedonic";
  propertyGroup: PropertyGroup;
  period: string;
  indexValue: number;
  sampleSize: number;
  basePeriod: string;
  lowerIndexValue?: number | null;
  upperIndexValue?: number | null;
  isPublishable: boolean;
  suppressionReason?: string | null;
  methodologyVersion?: string;
  modelVersion?: string | null;
  observedSampleSize?: number;
  referenceSampleSize?: number | null;
  nominalCoverage?: number | null;
  uncertaintyMethod?: string | null;
}

export interface StatisticalOutlier {
  transactionId: number;
  contractDate?: string | null;
  propertyGroup: PropertyGroup;
  area?: number | null;
  price?: number | null;
  pricePerSqm?: number | null;
  monthlyRent?: number | null;
  monthlyRentPerSqm?: number | null;
  cohortLevel: string;
  cohortSize: number;
  cohortMedian: number;
  lowerBound: number;
  upperBound: number;
  modifiedZScore?: number | null;
  ruleVersion: string;
}

export interface FullValuationPrediction extends ValuationPrediction {
  transactionKind: MarketMode;
  transactionId: number;
  propertyGroup: PropertyGroup;
  area: number;
  actualAmountPerSqm: number;
  logResidual: number;
  modelType: string;
  trainingRows: number;
  metrics: {
    r2_log?: number;
    rmse_log?: number;
    median_absolute_percentage_error?: number;
    empirical_interval_coverage?: number;
    property_groups?: Record<string, {
      temporal?: {
        r2_log?: number;
        rmse_log?: number;
        median_absolute_percentage_error?: number;
        empirical_interval_coverage?: number;
        mae_amount_per_sqm?: number;
      };
      spatial?: {
        r2_log?: number;
        rmse_log?: number;
        median_absolute_percentage_error?: number;
        held_out_municipalities?: number;
      };
    }>;
  };
}

export interface AnomalySummary {
  total: number;
  flagged: number;
  unreviewed: number;
  highPriority: number;
}

export interface TransactionContextResponse {
  transactionKind: MarketMode;
  transactionId: number;
  contractYear: number;
  incompletePeriod: boolean;
  propertyGroup: PropertyGroup;
  actualAmountPerSqm: number;
  directComparables: DirectComparable[];
  benchmarks: ContextBenchmark[];
  valuation?: ValuationPrediction | null;
  currentMarketEstimate?: CurrentMarketEstimate;
  quality: QualityDecision;
  outlierStatus?: string | null;
  methodologyVersion: string;
}

export interface CurrentMarketEstimate {
  lowerTotal: number;
  centralTotal: number;
  upperTotal: number;
  lowerAmountPerSqm: number;
  centralAmountPerSqm: number;
  upperAmountPerSqm: number;
  comparableCount: number;
  maximumRadiusMeters: number;
  areaTolerancePercent: number;
  constructionYearTolerance?: number;
  locationAdjustmentFactor: number;
  localAdjustmentSampleSize: number;
  referenceYear: number;
  confidence: "moderate" | "low" | "very_low";
  warnings: string[];
  methodologyVersion: string;
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

export interface BuildingTransaction {
  transactionId: number;
  contractDate?: string | null;
  propertyGroup: PropertyGroup;
  area?: number | null;
  amount?: number | null;
  amountPerSqm?: number | null;
}

export interface BuildingStatistics {
  cadastralMunicipalityCode: number;
  buildingNumber: string;
  constructionYear?: number | null;
  knownBuildingPartCount: number;
  longitude?: number | null;
  latitude?: number | null;
  sales: BuildingMarketSummary;
  rentals: BuildingMarketSummary;
  saleHistory: BuildingYearPoint[];
  rentalHistory: BuildingYearPoint[];
  saleTransactions: BuildingTransaction[];
  rentalTransactions: BuildingTransaction[];
}

export interface BuildingMarketSummary {
  transactionCount: number;
  representativeCount: number;
  medianAmount?: number | null;
  medianAmountPerSqm?: number | null;
  firstContractDate?: string | null;
  lastContractDate?: string | null;
}

export interface BuildingYearPoint {
  year: number;
  transactionCount: number;
  representativeCount: number;
  medianAmountPerSqm?: number | null;
}

export type MarketContext =
  | { kind: "country" }
  | { kind: "geography"; id: number; level: GeographyLevel; name: string }
  | { kind: "building"; cadastralCode: number; buildingNumber: string; label?: string }
  | { kind: "transaction"; mode: MarketMode; id: number };

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
  source: "live";
}
