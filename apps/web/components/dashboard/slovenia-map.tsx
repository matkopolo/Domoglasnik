"use client";

import { Layers3, LocateFixed, Mountain, Search, X } from "lucide-react";
import maplibregl, { type GeoJSONSource, type LngLatBoundsLike } from "maplibre-gl";
import { useCallback, useEffect, useRef, useState } from "react";
import { loadMapFeatures, loadRegionBoundaries, loadTransaction, mapGeographyLevel, searchBuildings } from "@/lib/api";
import { buildMapPopup, mapAmountLabel } from "@/lib/map-popup";
import { mapStyle, type MapStyleId } from "@/lib/map-styles";
import { propertyGroupLabels, suggestedBuildingMarket } from "@/lib/building-search";
import type {
  BoundaryFeatureCollection,
  AreaRange,
  BuildingSearchResult,
  Cluster,
  Geography,
  GeographyLevel,
  MapFeatureCollection,
  MapPointFeature,
  MarketContext,
  MarketMode,
  PropertyGroup,
  RentalTransactionDetail,
  SaleTransactionDetail,
  YearRange,
} from "@/lib/types";

const SLOVENIA_CENTER: [number, number] = [14.82, 46.12];
const SLOVENIA_BOUNDS: LngLatBoundsLike = [[13.25, 45.35], [16.75, 46.95]];
const EMPTY_BOUNDARIES: BoundaryFeatureCollection = { type: "FeatureCollection", features: [] };

function initialPoints(clusters: Cluster[]): MapFeatureCollection {
  return {
    type: "FeatureCollection",
    features: clusters.map((cluster, index) => ({
      type: "Feature",
      id: index,
      geometry: { type: "Point", coordinates: [cluster.longitude, cluster.latitude] },
      properties: { ...cluster, featureKind: "cluster" },
    })),
  };
}

function nextSemanticZoom(level: unknown) {
  return {
    country: 7.4,
    statistical_region: 9.4,
    municipality: 11.8,
    city_district: 14,
    local_community: 14,
  }[String(level)];
}

function zoomToSemanticFeature(map: maplibregl.Map, feature: MapPointFeature) {
  const coordinates = feature.geometry.coordinates as [number, number];
  const targetZoom = nextSemanticZoom(feature.properties.level);
  const rawBounds = feature.properties.bounds;
  if (Array.isArray(rawBounds) && rawBounds.length === 4) {
    const camera = map.cameraForBounds(
      [[Number(rawBounds[0]), Number(rawBounds[1])], [Number(rawBounds[2]), Number(rawBounds[3])]],
      { padding: 42 },
    );
    map.easeTo({
      center: camera?.center ?? coordinates,
      zoom: Math.max(camera?.zoom ?? map.getZoom() + 2, targetZoom ?? 0),
      duration: 700,
    });
  } else {
    map.easeTo({ center: coordinates, zoom: targetZoom ?? Math.min(map.getZoom() + 2, 14), duration: 700 });
  }
}

interface SloveniaMapProps {
  mode: MarketMode;
  context: MarketContext;
  propertyGroup: PropertyGroup;
  clusters: Cluster[];
  regions: Geography[];
  selectedId?: number;
  years: YearRange;
  area: AreaRange;
  refreshing?: boolean;
  onContextChange: (context: MarketContext) => void;
  onMarketSuggestion: (mode: MarketMode, propertyGroup: PropertyGroup) => void;
  resetVersion?: number;
}


export function SloveniaMap({
  mode,
  context,
  propertyGroup,
  clusters,
  regions,
  selectedId,
  years,
  area,
  refreshing,
  onContextChange,
  onMarketSuggestion,
  resetVersion = 0,
}: SloveniaMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const pointsRef = useRef<MapFeatureCollection>(initialPoints(clusters));
  const boundariesRef = useRef<BoundaryFeatureCollection>(EMPTY_BOUNDARIES);
  const regionsRef = useRef(regions);
  const markersRef = useRef<maplibregl.Marker[]>([]);
  const popupRef = useRef<maplibregl.Popup | null>(null);
  const selectionMarkerRef = useRef<maplibregl.Marker | null>(null);
  const contextRef = useRef(onContextChange);
  const modeRef = useRef(mode);
  const [styleId, setStyleId] = useState<MapStyleId>("street");
  const [mapStatus, setMapStatus] = useState<"ready" | "loading" | "offline">("loading");
  const [visiblePointCount, setVisiblePointCount] = useState(clusters.length);
  const [visibleLevel, setVisibleLevel] = useState<GeographyLevel | "property">("country");
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<BuildingSearchResult[]>([]);
  const [locationNotice, setLocationNotice] = useState<string>();
  const styleIdRef = useRef<MapStyleId>("street");
  const previousResetVersion = useRef(resetVersion);

  useEffect(() => { contextRef.current = onContextChange; }, [onContextChange]);
  useEffect(() => { modeRef.current = mode; }, [mode]);
  useEffect(() => { regionsRef.current = regions; }, [regions]);

  useEffect(() => {
    if (previousResetVersion.current === resetVersion) return;
    previousResetVersion.current = resetVersion;
    popupRef.current?.remove();
    mapRef.current?.fitBounds(SLOVENIA_BOUNDS, { padding: 20, duration: 650 });
  }, [resetVersion]);

  const renderMarkers = useCallback((data: MapFeatureCollection) => {
    const map = mapRef.current;
    if (!map) return;
    markersRef.current.forEach((marker) => marker.remove());
    markersRef.current = data.features.map((feature) => {
      const kind = String(feature.properties.featureKind);
      const isBuilding = kind === "building";
      const isProperty = kind === "property";
      const count = Number(feature.properties.transactionCount ?? 1);
      const element = document.createElement("button");
      element.type = "button";
      element.className = `market-map-marker ${modeRef.current} ${
        isBuilding || isProperty ? "property" : "cluster geography"
      } ${isBuilding ? "building" : ""}`;
      if (!isBuilding && !isProperty) element.dataset.level = String(feature.properties.level ?? "");
      element.ariaLabel = isBuilding
        ? `Odpri stavbo z ${count} posli`
        : isProperty
          ? `Odpri posel ${feature.properties.transactionId ?? ""}`
          : `Odpri območje ${feature.properties.name ?? ""}`;
      if (isBuilding) element.textContent = count.toLocaleString("sl-SI");
      if (!isBuilding && !isProperty) {
        element.textContent = String(feature.properties.name ?? "");
        const metric = mapAmountLabel(feature.properties, modeRef.current);
        if (metric) {
          const small = document.createElement("small");
          small.textContent = metric;
          element.append(small);
        }
      }
      element.addEventListener("click", (event) => {
        event.stopPropagation();
        popupRef.current?.remove();
        popupRef.current = new maplibregl.Popup({ offset: 18, closeButton: false })
          .setLngLat(feature.geometry.coordinates as [number, number])
          .setDOMContent(buildMapPopup(feature, modeRef.current))
          .addTo(map);
        if (isBuilding) {
          contextRef.current({
            kind: "building",
            cadastralCode: Number(feature.properties.cadastralMunicipalityCode),
            buildingNumber: String(feature.properties.buildingNumber),
          });
          map.easeTo({
            center: feature.geometry.coordinates as [number, number],
            zoom: Math.max(map.getZoom(), 16),
            duration: 500,
          });
        } else if (isProperty) {
          contextRef.current({
            kind: "transaction",
            mode: modeRef.current,
            id: Number(feature.properties.transactionId),
          });
        } else {
          const id = Number(feature.properties.geographyId ?? feature.properties.id);
          if (id) {
            contextRef.current({
              kind: "geography",
              id,
              level: String(feature.properties.level) as GeographyLevel,
              name: String(feature.properties.name),
            });
          }
          zoomToSemanticFeature(map, feature);
        }
      });
      return new maplibregl.Marker({ element, anchor: "center" })
        .setLngLat(feature.geometry.coordinates as [number, number])
        .addTo(map);
    });
  }, []);

  useEffect(() => {
    const query = searchQuery.trim();
    if (query.length < 3) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      searchBuildings(query, years, controller.signal)
        .then((response) => setSearchResults(response.items))
        .catch(() => { if (!controller.signal.aborted) setSearchResults([]); });
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [searchQuery, years]);

  function chooseBuilding(result: BuildingSearchResult) {
    popupRef.current?.remove();
    setSearchResults([]);
    setSearchQuery([
      result.street,
      [result.houseNumber, result.houseNumberSuffix].filter(Boolean).join(""),
      result.settlement,
    ].filter(Boolean).join(", "));
    mapRef.current?.easeTo({ center: [result.longitude, result.latitude], zoom: 17, duration: 850 });
    onContextChange({
      kind: "building",
      cadastralCode: result.cadastralMunicipalityCode,
      buildingNumber: result.buildingNumber,
      label: [result.street, result.houseNumber].filter(Boolean).join(" "),
    });
  }

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;
    const map = new maplibregl.Map({
      container: containerRef.current,
      style: mapStyle("street"),
      center: SLOVENIA_CENTER,
      zoom: 7.15,
      minZoom: 6.25,
      maxZoom: 18,
      maxBounds: SLOVENIA_BOUNDS,
      attributionControl: false,
    });
    mapRef.current = map;
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "bottom-left");
    map.addControl(new maplibregl.AttributionControl({ compact: true }), "bottom-right");
    map.on("style.load", () => {
      if (styleIdRef.current === "street") {
        for (const layer of map.getStyle().layers ?? []) {
          try {
            if (layer.type === "background") {
              map.setPaintProperty(layer.id, "background-color", "#ebe9e2");
            } else if (layer.type === "fill" && /water/i.test(layer.id)) {
              map.setPaintProperty(layer.id, "fill-color", "#d7e0df");
            } else if (layer.type === "fill" && /land|park|landcover/i.test(layer.id)) {
              map.setPaintProperty(layer.id, "fill-color", "#eeece5");
            } else if (layer.type === "line" && /road|highway|street|transport/i.test(layer.id)) {
              map.setPaintProperty(layer.id, "line-color", "#c9c9c2");
              map.setPaintProperty(layer.id, "line-opacity", 0.72);
            }
          } catch {
            // Provider styles differ; unsupported paint properties stay unchanged.
          }
        }
      }
      if (!map.getSource("regions")) {
        map.addSource("regions", { type: "geojson", data: boundariesRef.current });
        map.addLayer({
          id: "region-fill",
          type: "fill",
          source: "regions",
          paint: { "fill-color": "#0d5c3d", "fill-opacity": 0.045 },
        });
        map.addLayer({
          id: "region-outline",
          type: "line",
          source: "regions",
          paint: { "line-color": "#285c43", "line-width": 1.4, "line-opacity": 0.8 },
        });
      }
      renderMarkers(pointsRef.current);
      setMapStatus("ready");
    });
    map.on("error", () => setMapStatus("offline"));
    map.on("click", (event) => {
      const features = map.getLayer("region-fill")
        ? map.queryRenderedFeatures(event.point, { layers: ["region-fill"] })
        : [];
      const id = Number(features[0]?.properties?.id);
      const region = regionsRef.current.find((item) => item.id === id);
      if (region) contextRef.current({
        kind: "geography",
        id: region.id,
        level: region.level,
        name: region.name,
      });
    });
    return () => {
      popupRef.current?.remove();
      markersRef.current.forEach((marker) => marker.remove());
      map.remove();
      mapRef.current = null;
    };
  }, [renderMarkers]);

  useEffect(() => {
    pointsRef.current = initialPoints(clusters);
  }, [clusters]);

  useEffect(() => {
    selectionMarkerRef.current?.remove();
    selectionMarkerRef.current = null;
    queueMicrotask(() => setLocationNotice(undefined));
    if (context.kind !== "transaction") return;
    const map = mapRef.current;
    if (!map) return;
    const controller = new AbortController();
    loadTransaction(context.mode, context.id, controller.signal).then((detail) => {
      if (controller.signal.aborted) return;
      if (detail.longitude == null || detail.latitude == null) {
        setLocationNotice(`Posel #${context.id} nima uporabnih koordinat.`);
        return;
      }
      const coordinates: [number, number] = [detail.longitude, detail.latitude];
      const marker = document.createElement("button");
      marker.type = "button";
      marker.className = `market-map-marker ${context.mode} property transaction-focus`;
      marker.ariaLabel = `Izbrani ${context.mode === "sale" ? "prodajni" : "najemni"} posel ${context.id}`;
      marker.textContent = "●";
      selectionMarkerRef.current = new maplibregl.Marker({ element: marker, anchor: "center" })
        .setLngLat(coordinates)
        .addTo(map);
      const properties = context.mode === "sale"
        ? (() => {
            const sale = detail as SaleTransactionDetail;
            return {
              featureKind: "property", transactionId: sale.transactionId,
              area: sale.area ?? null, price: sale.price ?? null,
              pricePerSqm: sale.pricePerSqm ?? null,
            };
          })()
        : (() => {
            const rental = detail as RentalTransactionDetail;
            return {
              featureKind: "property", transactionId: rental.transactionId,
              area: rental.area ?? null,
              monthlyRentPerSqm: rental.monthlyRentPerSqm ?? null,
            };
          })();
      const feature: MapPointFeature = {
        type: "Feature",
        geometry: { type: "Point", coordinates },
        properties,
      };
      popupRef.current?.remove();
      popupRef.current = new maplibregl.Popup({ offset: 18, closeButton: false })
        .setLngLat(coordinates)
        .setDOMContent(buildMapPopup(feature, context.mode))
        .addTo(map);
      map.easeTo({ center: coordinates, zoom: 17, duration: 850 });
    }).catch(() => {
      if (!controller.signal.aborted) {
        setLocationNotice(`Lokacije posla #${context.id} ni bilo mogoče naložiti.`);
      }
    });
    return () => controller.abort();
  }, [context]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    let controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    let generation = 0;
    const refresh = () => {
      clearTimeout(timer);
      controller.abort();
      controller = new AbortController();
      const requestGeneration = ++generation;
      timer = setTimeout(() => {
        const bounds = map.getBounds();
        const nextLevel = map.getZoom() >= 13.5 ? "property" : mapGeographyLevel(map.getZoom());
        setVisibleLevel(nextLevel);
        setMapStatus("loading");
        loadMapFeatures(
          mode,
          propertyGroup,
          selectedId,
          years,
          [bounds.getWest(), bounds.getSouth(), bounds.getEast(), bounds.getNorth()],
          map.getZoom(),
          controller.signal,
          area,
        ).then((data) => {
          if (requestGeneration !== generation) return;
          const returnedLevel = data.features[0]?.properties.level;
          if (nextLevel !== "property" && typeof returnedLevel === "string") {
            setVisibleLevel(returnedLevel as GeographyLevel);
          }
          pointsRef.current = data;
          setVisiblePointCount(data.features.length);
          renderMarkers(data);
          setMapStatus("ready");
        }).catch(() => {
          if (!controller.signal.aborted && requestGeneration === generation) setMapStatus("offline");
        });
      }, 180);
    };
    map.on("moveend", refresh);
    refresh();
    return () => {
      clearTimeout(timer);
      controller.abort();
      map.off("moveend", refresh);
    };
  }, [area, mode, propertyGroup, renderMarkers, selectedId, years]);

  useEffect(() => {
    if (regions.length === 0) return;
    const controller = new AbortController();
    loadRegionBoundaries(regions, controller.signal).then((data) => {
      boundariesRef.current = data;
      (mapRef.current?.getSource("regions") as GeoJSONSource | undefined)?.setData(data);
    }).catch(() => undefined);
    return () => controller.abort();
  }, [regions]);

  function changeStyle(next: MapStyleId) {
    setStyleId(next);
    styleIdRef.current = next;
    setMapStatus("loading");
    mapRef.current?.setStyle(mapStyle(next));
  }

  const visibleLevelLabel = {
    country: "Slovenija",
    statistical_region: "statistične regije",
    municipality: "občine",
    city_district: "mestne četrti in krajevna območja",
    local_community: "krajevna območja",
    settlement: "naselja",
    property: "stavbe in posli",
  }[visibleLevel];

  return (
    <section className={`map-panel real-map-panel ${context.kind === "building" || context.kind === "transaction" ? "map-focus-mode" : ""}`} aria-labelledby="map-title">
      <header className="panel-heading map-heading">
        <div><h2 id="map-title">Poiščite lokacijo ali raziščite trg</h2></div>
        <div className="map-key"><span className={`legend-dot ${mode}`} />
          <span role="status">{refreshing || mapStatus === "loading" ? "Osvežujem zemljevid …"
            : mapStatus === "offline" ? "Zemljevid ni dosegljiv"
              : visiblePointCount === 0 ? "Ni prikazanih poslov"
                : `${visibleLevelLabel} · ${visiblePointCount.toLocaleString("sl-SI")} prikazanih`}</span>
        </div>
      </header>
      <div ref={containerRef} className="maplibre-canvas" aria-label="Interaktivni zemljevid nepremičninskih poslov v Sloveniji" />
      {locationNotice && <div className="map-location-notice" role="status">{locationNotice}</div>}
      <div className="building-search">
        <label><Search size={16} /><span className="sr-only">Poišči stavbo</span>
          <input id="building-address-search" aria-label="Naslov stavbe" value={searchQuery} onChange={(event) => {
            setSearchQuery(event.target.value);
            if (event.target.value.trim().length < 3) setSearchResults([]);
          }} placeholder="Vnesite naslov" />
          {searchQuery && <button type="button" aria-label="Počisti iskanje"
            onClick={() => { setSearchQuery(""); setSearchResults([]); }}><X size={15} /></button>}
        </label>
        {searchResults.length > 0 && <div className="building-search-results">
          {searchResults.map((result) => {
            const activeCount = mode === "sale"
              ? result.saleCounts[propertyGroup] ?? 0
              : result.rentalCounts[propertyGroup] ?? 0;
            const suggestion = suggestedBuildingMarket(result, mode, propertyGroup);
            return <div className="building-search-result" key={`${result.cadastralMunicipalityCode}-${result.buildingNumber}`}>
              <button type="button" onClick={() => chooseBuilding(result)}>
                <strong>{[result.street, result.houseNumber].filter(Boolean).join(" ")}</strong>
                <span>{result.settlement} · {activeCount.toLocaleString("sl-SI")} poslov ({mode === "sale" ? "prodaja" : "najem"} · {propertyGroupLabels[propertyGroup]})</span>
              </button>
              {suggestion && <button type="button" className="building-filter-suggestion" onClick={() => {
                onMarketSuggestion(suggestion.mode, suggestion.group);
                chooseBuilding(result);
              }}>Ni poslov za trenutni filter. Prikaži: {suggestion.mode === "sale" ? "prodaja" : "najem"} · {propertyGroupLabels[suggestion.group]} ({suggestion.count.toLocaleString("sl-SI")})</button>}
            </div>;
          })}
        </div>}
      </div>
      <div className="map-style-control" role="group" aria-label="Vrsta zemljevida">
        <button type="button" className={styleId === "street" ? "active" : ""} onClick={() => changeStyle("street")}>
          <Layers3 size={15} /> Ceste
        </button>
        <button type="button" className={styleId === "satellite" ? "active" : ""} onClick={() => changeStyle("satellite")}>
          <Mountain size={15} /> Satelit
        </button>
      </div>
      <button type="button" className="map-home-control" aria-label="Prikaži celo Slovenijo"
        onClick={() => {
          onContextChange({ kind: "country" });
          mapRef.current?.fitBounds(SLOVENIA_BOUNDS, { padding: 20, duration: 650 });
        }}><LocateFixed size={17} /></button>
    </section>
  );
}
