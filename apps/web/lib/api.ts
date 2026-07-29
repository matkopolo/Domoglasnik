import type {
  BoundaryFeature,
  BoundaryFeatureCollection,
  BuildingSearchResult,
  BuildingStatistics,
  Cluster,
  DashboardData,
  Distribution,
  Geography,
  GeographyComparison,
  MapFeatureCollection,
  MarketMode,
  Overview,
  PropertyGroup,
  RentalTransactionDetail,
  SaleTransactionDetail,
  TimePoint,
  TransactionContextResponse,
  YearRange,
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

export class APIError extends Error {
  constructor(
    public readonly status: number,
    public readonly path: string,
  ) {
    super(`API request failed (${status})`);
    this.name = "APIError";
  }
}

export function marketQuery(
  propertyGroup: PropertyGroup,
  years: YearRange,
  geographyId?: number,
) {
  const query = new URLSearchParams({
    from: `${years.from}-01-01`,
    to: `${years.to}-12-31`,
    propertyGroup,
  });
  if (geographyId) query.set("geographyId", String(geographyId));
  return query;
}

export async function getJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    signal,
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new APIError(response.status, path);
  return response.json() as Promise<T>;
}

export async function loadDashboard(
  mode: MarketMode,
  propertyGroup: PropertyGroup,
  geographyId: number | undefined,
  years: YearRange,
  signal: AbortSignal,
): Promise<DashboardData> {
  const query = marketQuery(propertyGroup, years, geographyId);
  const prefix = mode === "sale" ? "" : "/rentals";
  const distributionMetric = mode === "sale" ? "pricePerSqm" : "rentPerSqm";
  const mapPath = mode === "sale" ? "/api/v1/map/clusters" : "/api/v1/rentals/map/clusters";
  const [overview, timeSeries, distribution, geographyResponse, clusterResponse] = await Promise.all([
    getJSON<Overview>(`/api/v1${prefix}/statistics/overview?${query}`, signal),
    getJSON<{ items: TimePoint[] }>(
      `/api/v1${prefix}/statistics/timeseries?${query}&interval=year`,
      signal,
    ),
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
  const query = marketQuery(propertyGroup, years, geographyId);
  query.set("bbox", bounds.join(","));
  const mapPrefix = mode === "sale" ? "/api/v1/map" : "/api/v1/rentals/map";
  const showProperties = zoom >= 13.5;
  if (showProperties) query.set("limit", "500");
  else {
    query.set("level", zoom < 7
      ? "country"
      : zoom < 9
        ? "statistical_region"
        : zoom < 11.5
          ? "municipality"
          : "city_district");
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
  if (!showProperties) return decorated;

  const grouped = new Map<string, MapFeatureCollection["features"][number]>();
  decorated.features.forEach((feature) => {
    const code = feature.properties.cadastralMunicipalityCode;
    const number = feature.properties.buildingNumber;
    const key = code && number
      ? `${code}:${number}`
      : `transaction:${feature.properties.transactionId ?? feature.id}`;
    const existing = grouped.get(key);
    if (!existing) {
      grouped.set(key, {
        ...feature,
        id: key,
        properties: {
          ...feature.properties,
          featureKind: code && number ? "building" : "property",
          transactionCount: 1,
        },
      });
    } else {
      existing.properties.transactionCount = Number(existing.properties.transactionCount ?? 1) + 1;
    }
  });
  return { ...decorated, features: [...grouped.values()] };
}

export function loadTransaction(mode: MarketMode, id: number, signal: AbortSignal) {
  return mode === "sale"
    ? getJSON<SaleTransactionDetail>(`/api/v1/sales/transactions/${id}`, signal)
    : getJSON<RentalTransactionDetail>(`/api/v1/rentals/transactions/${id}`, signal);
}

export function loadTransactionContext(mode: MarketMode, id: number, signal: AbortSignal) {
  const kind = mode === "sale" ? "sales" : "rentals";
  return getJSON<TransactionContextResponse>(
    `/api/v1/${kind}/transactions/${id}/context`,
    signal,
  );
}

export function searchBuildings(query: string, signal: AbortSignal) {
  return getJSON<{ items: BuildingSearchResult[] }>(
    `/api/v1/buildings/search?q=${encodeURIComponent(query)}&limit=10`,
    signal,
  );
}

export function loadBuildingStatistics(
  cadastralCode: number,
  buildingNumber: string,
  propertyGroup: PropertyGroup,
  years: YearRange,
  signal: AbortSignal,
) {
  const query = marketQuery(propertyGroup, years);
  return getJSON<BuildingStatistics>(
    `/api/v1/buildings/${cadastralCode}/${encodeURIComponent(buildingNumber)}/statistics?${query}`,
    signal,
  );
}

export function loadGeography(id: number, signal: AbortSignal) {
  return getJSON<Geography>(`/api/v1/geographies/${id}`, signal);
}

export function loadGeographyChildren(id: number, signal: AbortSignal) {
  return getJSON<{ items: Geography[] }>(`/api/v1/geographies/${id}/children`, signal);
}

export async function loadGeographyComparisons(
  mode: MarketMode,
  parent: Geography,
  children: Geography[],
  propertyGroup: PropertyGroup,
  years: YearRange,
  signal: AbortSignal,
): Promise<GeographyComparison[]> {
  if (children.length === 0 || !parent.bounds) return [];
  const query = marketQuery(propertyGroup, years, parent.id);
  query.set("bbox", parent.bounds.join(","));
  query.set("level", children[0].level);
  const prefix = mode === "sale" ? "/api/v1/map" : "/api/v1/rentals/map";
  const response = await getJSON<MapFeatureCollection>(
    `${prefix}/geographies?${query}`,
    signal,
  );
  return response.features.map((feature) => ({
    id: Number(feature.properties.geographyId ?? feature.properties.id),
    name: String(feature.properties.name),
    level: String(feature.properties.level) as Geography["level"],
    transactionCount: Number(feature.properties.transactionCount ?? 0),
    representativeCount: Number(feature.properties.representativeCount ?? 0),
    medianAmountPerSqm: typeof feature.properties.medianAmountPerSqm === "number"
      ? feature.properties.medianAmountPerSqm
      : undefined,
  }));
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
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");
  return {
    type: "FeatureCollection",
    features: results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []),
  };
}
