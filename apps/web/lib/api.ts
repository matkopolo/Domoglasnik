import { demoDashboard } from "./demo-data";
import type {
  BoundaryFeature,
  BoundaryFeatureCollection,
  BuildingSearchResult,
  BuildingStatistics,
  Cluster,
  DashboardData,
  Distribution,
  Geography,
  MapFeatureCollection,
  MarketMode,
  Overview,
  PropertyGroup,
  SaleTransactionDetail,
  TimePoint,
  YearRange,
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

function queryString(propertyGroup: PropertyGroup, years: YearRange, geographyId?: number) {
  const query = new URLSearchParams({
    from: `${years.from}-01-01`,
    to: `${years.to}-12-31`,
    propertyGroup,
  });
  if (geographyId) query.set("geographyId", String(geographyId));
  return query;
}

async function getJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`API request failed (${response.status})`);
  return response.json() as Promise<T>;
}

export async function loadDashboard(
  mode: MarketMode,
  propertyGroup: PropertyGroup,
  geographyId: number | undefined,
  years: YearRange,
  signal: AbortSignal,
): Promise<DashboardData> {
  const query = queryString(propertyGroup, years, geographyId);
  const prefix = mode === "sale" ? "" : "/rentals";
  const distributionMetric = mode === "sale" ? "pricePerSqm" : "rentPerSqm";
  const mapPath = mode === "sale" ? "/api/v1/map/clusters" : "/api/v1/rentals/map/clusters";
  try {
    const [overview, timeSeries, distribution, geographyResponse, clusterResponse] = await Promise.all([
      getJSON<Overview>(`/api/v1${prefix}/statistics/overview?${query}`, signal),
      getJSON<{ items: TimePoint[] }>(`/api/v1${prefix}/statistics/timeseries?${query}&interval=year`, signal),
      getJSON<Distribution>(
        `/api/v1${prefix}/statistics/distribution?${query}&metric=${distributionMetric}&bins=12`,
        signal,
      ),
      getJSON<{ items: Geography[] }>("/api/v1/geographies?level=statistical_region", signal),
      getJSON<{ features: Array<{ geometry: { coordinates: [number, number] }; properties: Cluster }> }>(
        `${mapPath}?bbox=13.3,45.4,16.7,46.9&zoom=8&${query}`,
        signal,
      ),
    ]);
    return {
      overview,
      timeSeries: timeSeries.items,
      distribution,
      geographies: geographyResponse.items,
      clusters: clusterResponse.features.map((feature) => ({
        ...feature.properties,
        longitude: feature.geometry.coordinates[0],
        latitude: feature.geometry.coordinates[1],
      })),
      source: "live",
    };
  } catch (error) {
    if (signal.aborted) throw error;
    return demoDashboard(mode);
  }
}

export async function loadMapFeatures(
  mode: MarketMode,
  propertyGroup: PropertyGroup,
  geographyId: number | undefined,
  years: YearRange,
  bounds: [number, number, number, number],
  zoom: number,
  signal: AbortSignal,
): Promise<MapFeatureCollection> {
  const query = queryString(propertyGroup, years, geographyId);
  query.set("bbox", bounds.join(","));
  const mapPrefix = mode === "sale" ? "/api/v1/map" : "/api/v1/rentals/map";
  const showProperties = zoom >= 13.5;
  if (showProperties) query.set("limit", "500");
  else {
    const level = zoom < 7
      ? "country"
      : zoom < 9
        ? "statistical_region"
        : zoom < 11.5
          ? "municipality"
          : "city_district";
    query.set("level", level);
  }
  let response = await getJSON<MapFeatureCollection>(
    `${mapPrefix}/${showProperties ? "properties" : "geographies"}?${query}`,
    signal,
  );
  if (!showProperties && query.get("level") === "city_district" && response.features.length === 0) {
    query.set("level", "local_community");
    response = await getJSON<MapFeatureCollection>(`${mapPrefix}/geographies?${query}`, signal);
  }
  const decorated: MapFeatureCollection = {
    ...response,
    features: response.features.map((feature) => ({
      ...feature,
      properties: {
        ...feature.properties,
        featureKind: showProperties ? "property" : "geography",
      },
    })),
  };
  if (!showProperties || mode !== "sale") return decorated;

  const grouped = new Map<string, MapFeatureCollection["features"][number]>();
  decorated.features.forEach((feature) => {
    const code = feature.properties.cadastralMunicipalityCode;
    const buildingNumber = feature.properties.buildingNumber;
    const key = code && buildingNumber
      ? `${code}:${buildingNumber}`
      : `transaction:${feature.properties.transactionId ?? feature.id}`;
    const existing = grouped.get(key);
    if (!existing) {
      grouped.set(key, {
        ...feature,
        id: key,
        properties: {
          ...feature.properties,
          featureKind: code && buildingNumber ? "building" : "property",
          transactionCount: 1,
        },
      });
      return;
    }
    existing.properties.transactionCount = Number(existing.properties.transactionCount ?? 1) + 1;
  });
  return { ...decorated, features: [...grouped.values()] };
}

export function loadSaleTransaction(id: number, signal: AbortSignal) {
  return getJSON<SaleTransactionDetail>(`/api/v1/sales/transactions/${id}`, signal);
}

export async function searchBuildings(query: string, signal: AbortSignal) {
  return getJSON<{ items: BuildingSearchResult[] }>(
    `/api/v1/buildings/search?q=${encodeURIComponent(query)}&limit=10`,
    signal,
  );
}

export function loadBuildingStatistics(
  cadastralCode: number,
  buildingNumber: string,
  signal: AbortSignal,
) {
  return getJSON<BuildingStatistics>(
    `/api/v1/buildings/${cadastralCode}/${encodeURIComponent(buildingNumber)}/statistics`,
    signal,
  );
}

export async function loadRegionBoundaries(
  regions: Geography[],
  signal: AbortSignal,
): Promise<BoundaryFeatureCollection> {
  const results = await Promise.allSettled(
    regions.map((region) =>
      getJSON<BoundaryFeature>(`/api/v1/geographies/${region.id}/boundary?detail=low`, signal),
    ),
  );
  return {
    type: "FeatureCollection",
    features: results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []),
  };
}
