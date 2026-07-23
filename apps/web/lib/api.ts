import { demoDashboard } from "./demo-data";
import type { Cluster, DashboardData, Distribution, Geography, MarketMode, Overview, PropertyGroup, TimePoint } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

function queryString(mode: MarketMode, propertyGroup: PropertyGroup, geographyId?: number) {
  const query = new URLSearchParams({
    from: "2018-01-01",
    to: "2025-12-31",
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
  signal: AbortSignal,
): Promise<DashboardData> {
  const query = queryString(mode, propertyGroup, geographyId);
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
