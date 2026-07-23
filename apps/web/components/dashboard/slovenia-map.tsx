"use client";

import {
  Layers3,
  LocateFixed,
  Mountain,
  Search,
  X,
} from "lucide-react";
import maplibregl, {
  type GeoJSONSource,
  type LngLatBoundsLike,
} from "maplibre-gl";
import { useEffect, useRef, useState } from "react";
import {
  loadBuildingStatistics,
  loadMapFeatures,
  loadRegionBoundaries,
  loadSaleTransaction,
  searchBuildings,
} from "@/lib/api";
import { mapStyle, type MapStyleId } from "@/lib/map-styles";
import type {
  BoundaryFeatureCollection,
  BuildingSearchResult,
  BuildingStatistics,
  Cluster,
  Geography,
  MapFeatureCollection,
  MapPointFeature,
  MarketMode,
  PropertyGroup,
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

function amountLabel(properties: MapPointFeature["properties"], mode: MarketMode) {
  const value = mode === "sale"
    ? properties?.medianPricePerSqm ?? properties?.pricePerSqm
    : properties?.medianMonthlyRentPerSqm ?? properties?.monthlyRentPerSqm;
  return typeof value === "number"
    ? `${new Intl.NumberFormat("sl-SI", { maximumFractionDigits: 0 }).format(value)} €/m²`
    : "Ni podatka o ceni";
}

function popupContent(feature: MapPointFeature, mode: MarketMode) {
  const wrapper = document.createElement("div");
  wrapper.className = "map-popup";
  const properties = feature.properties ?? {};
  const isProperty = properties.featureKind === "property";
  const title = document.createElement("strong");
  title.textContent = isProperty
    ? `${mode === "sale" ? "Prodaja" : "Najem"} #${properties.transactionId ?? feature.id ?? ""}`
    : String(properties.name ?? `${Number(properties.transactionCount ?? 0).toLocaleString("sl-SI")} poslov`);
  const metric = document.createElement("span");
  metric.textContent = amountLabel(properties, mode);
  wrapper.append(title, metric);
  if (!isProperty) {
    const count = document.createElement("small");
    count.textContent = `${Number(properties.transactionCount ?? 0).toLocaleString("sl-SI")} poslov`;
    wrapper.append(count);
  }
  if (isProperty && typeof properties.area === "number") {
    const area = document.createElement("span");
    area.textContent = `${Number(properties.area).toLocaleString("sl-SI")} m²`;
    wrapper.append(area);
  }
  if (isProperty && mode === "sale" && typeof properties.price === "number") {
    const price = document.createElement("span");
    price.textContent = `${Number(properties.price).toLocaleString("sl-SI")} € skupaj`;
    wrapper.append(price);
  }
  if (isProperty && properties.contractDate) {
    const date = document.createElement("small");
    date.textContent = String(properties.contractDate);
    wrapper.append(date);
  }
  return wrapper;
}

function appendSaleDetail(wrapper: HTMLElement, detail: SaleTransactionDetail) {
  const part = detail.buildingParts?.[0];
  wrapper.replaceChildren();
  const title = document.createElement("strong");
  title.textContent = `Prodaja #${detail.transactionId}`;
  wrapper.append(title);
  const rows: Array<[string, string]> = [
    ["Leto gradnje", part?.constructionYear ? String(part.constructionYear) : "Ni podatka"],
    ["Cena", detail.price == null ? "Ni podatka" : `${detail.price.toLocaleString("sl-SI")} €`],
    ["Cena na m²", detail.pricePerSqm == null ? "Ni podatka" : `${detail.pricePerSqm.toLocaleString("sl-SI", { maximumFractionDigits: 2 })} €/m²`],
    ["Velikost", detail.area == null ? "Ni podatka" : `${detail.area.toLocaleString("sl-SI", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} m²`],
    ["Uporabna površina", part?.usableArea == null ? "Ni podatka" : `${part.usableArea.toLocaleString("sl-SI", { maximumFractionDigits: 2 })} m²`],
    ["Nadstropje", part?.floor?.trim() || "Ni podatka"],
  ];
  rows.forEach(([label, value]) => {
    const row = document.createElement("span");
    row.className = "detail-row";
    row.textContent = `${label}: ${value}`;
    wrapper.append(row);
  });
  if (part?.positionInBuilding) {
    const row = document.createElement("span");
    row.className = "detail-row";
    row.textContent = `Položaj: ${part.positionInBuilding}`;
    wrapper.append(row);
  }
  const address = [part?.street, part?.houseNumber, part?.settlement].filter(Boolean).join(", ");
  if (address) {
    const row = document.createElement("small");
    row.textContent = address;
    wrapper.append(row);
  }
  if ((detail.buildingParts?.length ?? 0) > 1) {
    const row = document.createElement("small");
    const extraArea = detail.buildingParts.slice(1)
      .reduce((sum, item) => sum + Number(item.soldArea ?? 0), 0);
    const positions = detail.buildingParts.slice(1)
      .map((item) => item.positionInBuilding)
      .filter(Boolean)
      .join(", ");
    row.textContent = `Dodatni deli: ${detail.buildingParts.length - 1}${extraArea ? ` · ${extraArea.toLocaleString("sl-SI")} m²` : ""}${positions ? ` · ${positions}` : ""}`;
    wrapper.append(row);
  }
  if ((detail.buildingParts?.length ?? 0) + (detail.landParcels?.length ?? 0) > 1) {
    const note = document.createElement("small");
    note.textContent = "Cena na m² uporablja skupno ceno paketa in referenčno površino glavne nepremičnine.";
    wrapper.append(note);
  }
}

function appendBuildingDetail(
  wrapper: HTMLElement,
  building: BuildingStatistics,
  map: maplibregl.Map,
  coordinates: [number, number],
) {
  wrapper.replaceChildren();
  const title = document.createElement("strong");
  title.textContent = `Stavba ${building.buildingNumber}`;
  const summary = document.createElement("span");
  summary.textContent = `${building.sales.transactionCount.toLocaleString("sl-SI")} prodaj`;
  wrapper.append(title, summary);
  building.saleTransactions.forEach((transaction) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "popup-transaction";
    button.textContent = `#${transaction.transactionId} · ${transaction.contractDate ?? "brez datuma"} · ${transaction.amount?.toLocaleString("sl-SI") ?? "—"} € · ${transaction.area?.toLocaleString("sl-SI") ?? "—"} m² · ${transaction.amountPerSqm?.toLocaleString("sl-SI", { maximumFractionDigits: 2 }) ?? "—"} €/m²`;
    button.addEventListener("click", () => {
      const controller = new AbortController();
      loadSaleTransaction(transaction.transactionId, controller.signal)
        .then((detail) => appendSaleDetail(wrapper, detail))
        .catch(() => undefined);
    });
    wrapper.append(button);
  });
  if (building.saleTransactions.length === 0) {
    const empty = document.createElement("small");
    empty.textContent = "Za stavbo ni prodajnih poslov.";
    wrapper.append(empty);
  }
  map.easeTo({ center: coordinates, zoom: Math.max(map.getZoom(), 17), duration: 500 });
}

function nextSemanticZoom(level: unknown) {
  switch (level) {
    case "country": return 7.4;
    case "statistical_region": return 9.4;
    case "municipality": return 11.8;
    case "city_district":
    case "local_community": return 14;
    default: return undefined;
  }
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
    return;
  }
  map.easeTo({
    center: coordinates,
    zoom: targetZoom ?? Math.min(map.getZoom() + 2, 14),
    duration: 700,
  });
}

interface SloveniaMapProps {
  mode: MarketMode;
  propertyGroup: PropertyGroup;
  clusters: Cluster[];
  regions: Geography[];
  selectedId?: number;
  years: YearRange;
  onSelect: (id?: number) => void;
}

export function SloveniaMap({
  mode,
  propertyGroup,
  clusters,
  regions,
  selectedId,
  years,
  onSelect,
}: SloveniaMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const pointsRef = useRef<MapFeatureCollection>(initialPoints(clusters));
  const boundariesRef = useRef<BoundaryFeatureCollection>(EMPTY_BOUNDARIES);
  const markersRef = useRef<maplibregl.Marker[]>([]);
  const popupRef = useRef<maplibregl.Popup | null>(null);
  const modeRef = useRef(mode);
  const onSelectRef = useRef(onSelect);
  const [styleId, setStyleId] = useState<MapStyleId>("street");
  const [mapStatus, setMapStatus] = useState<"ready" | "loading" | "offline">("loading");
  const [visiblePointCount, setVisiblePointCount] = useState(clusters.length);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<BuildingSearchResult[]>([]);
  const [selectedBuilding, setSelectedBuilding] = useState<BuildingStatistics>();

  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);

  useEffect(() => {
    onSelectRef.current = onSelect;
  }, [onSelect]);

  useEffect(() => {
    const query = searchQuery.trim();
    if (query.length < 3) {
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      searchBuildings(query, controller.signal)
        .then((response) => setSearchResults(response.items))
        .catch(() => setSearchResults([]));
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [searchQuery]);

  function chooseBuilding(result: BuildingSearchResult) {
    popupRef.current?.remove();
    popupRef.current = null;
    setSearchResults([]);
    setSearchQuery([
      result.street,
      [result.houseNumber, result.houseNumberSuffix].filter(Boolean).join(""),
      result.settlement,
    ].filter(Boolean).join(", "));
    mapRef.current?.easeTo({
      center: [result.longitude, result.latitude],
      zoom: 17,
      duration: 850,
    });
    const controller = new AbortController();
    loadBuildingStatistics(
      result.cadastralMunicipalityCode,
      result.buildingNumber,
      controller.signal,
    ).then(setSelectedBuilding).catch(() => setSelectedBuilding(undefined));
  }

  function openBuildingTransaction(transactionId: number) {
    const map = mapRef.current;
    if (!map || selectedBuilding?.longitude == null || selectedBuilding.latitude == null) return;
    const content = document.createElement("div");
    content.className = "map-popup";
    content.textContent = "Nalagam podatke …";
    const controller = new AbortController();
    popupRef.current?.remove();
    const popup = new maplibregl.Popup({ offset: 18 })
      .setLngLat([selectedBuilding.longitude, selectedBuilding.latitude])
      .setDOMContent(content)
      .addTo(map);
    popupRef.current = popup;
    popup.on("close", () => {
      controller.abort();
      if (popupRef.current === popup) popupRef.current = null;
    });
    loadSaleTransaction(transactionId, controller.signal)
      .then((detail) => appendSaleDetail(content, detail))
      .catch(() => { content.textContent = "Podatkov ni bilo mogoče naložiti."; });
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

    function syncLayers() {
      if (!map.isStyleLoaded()) return;
      if (!map.getSource("regions")) {
        map.addSource("regions", { type: "geojson", data: boundariesRef.current });
        map.addLayer({
          id: "region-fill",
          type: "fill",
          source: "regions",
          paint: {
            "fill-color": "#0d5c3d",
            "fill-opacity": ["case", ["boolean", ["feature-state", "selected"], false], 0.17, 0.045],
          },
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
    }

    function renderMarkers(data: MapFeatureCollection) {
      markersRef.current.forEach((marker) => marker.remove());
      markersRef.current = data.features.map((feature) => {
        const isProperty = feature.properties.featureKind === "property";
        const isBuilding = feature.properties.featureKind === "building";
        const count = Number(feature.properties.transactionCount ?? 1);
        const element = document.createElement("button");
        element.type = "button";
        element.className = `market-map-marker ${modeRef.current} ${isProperty || isBuilding ? "property" : "cluster"} ${isBuilding ? "building" : ""}`;
        element.ariaLabel = isBuilding
          ? `Odpri stavbo z ${count} prikazanimi posli`
          : isProperty
          ? `Odpri posel ${feature.properties.transactionId ?? feature.id ?? ""}`
          : `Odpri skupino ${count} poslov`;
        if (isBuilding) element.textContent = count.toLocaleString("sl-SI");
        if (!isProperty && !isBuilding) {
          element.classList.add("geography");
          element.textContent = String(feature.properties.name ?? "");
          const amount = document.createElement("small");
          amount.textContent = amountLabel(feature.properties, modeRef.current);
          element.append(amount);
        }
        element.addEventListener("click", (event) => {
          event.stopPropagation();
          const coordinates = feature.geometry.coordinates as [number, number];
          const content = popupContent(feature, modeRef.current);
          popupRef.current?.remove();
          const popup = new maplibregl.Popup({ offset: 18, closeButton: false })
            .setLngLat(coordinates)
            .setDOMContent(content)
            .addTo(map);
          popupRef.current = popup;
          popup.on("close", () => {
            if (popupRef.current === popup) popupRef.current = null;
          });
          if (isBuilding && feature.properties.cadastralMunicipalityCode && feature.properties.buildingNumber) {
            const controller = new AbortController();
            popup.on("close", () => controller.abort());
            loadBuildingStatistics(
              Number(feature.properties.cadastralMunicipalityCode),
              String(feature.properties.buildingNumber),
              controller.signal,
            ).then((building) => appendBuildingDetail(content, building, map, coordinates))
              .catch(() => undefined);
          }
          if (isProperty && modeRef.current === "sale" && feature.properties.transactionId) {
            const controller = new AbortController();
            popup.on("close", () => controller.abort());
            loadSaleTransaction(Number(feature.properties.transactionId), controller.signal)
              .then((detail) => appendSaleDetail(content, detail))
              .catch(() => undefined);
          }
          if (!isProperty && !isBuilding) {
            zoomToSemanticFeature(map, feature);
          }
        });
        return new maplibregl.Marker({ element, anchor: "center" })
          .setLngLat(feature.geometry.coordinates as [number, number])
          .addTo(map);
      });
    }

    map.on("style.load", syncLayers);
    map.on("error", () => setMapStatus("offline"));
    map.on("mouseenter", "region-fill", () => { map.getCanvas().style.cursor = "pointer"; });
    map.on("mouseleave", "region-fill", () => { map.getCanvas().style.cursor = ""; });
    map.on("click", (event) => {
      const regionFeatures = map.getLayer("region-fill")
        ? map.queryRenderedFeatures(event.point, { layers: ["region-fill"] })
        : [];
      const regionId = Number(regionFeatures[0]?.properties?.id);
      if (regionId) onSelectRef.current(regionId);
    });
    return () => {
      popupRef.current?.remove();
      popupRef.current = null;
      markersRef.current.forEach((marker) => marker.remove());
      markersRef.current = [];
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    pointsRef.current = initialPoints(clusters);
  }, [clusters]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const refresh = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        const bounds = map.getBounds();
        setMapStatus("loading");
        loadMapFeatures(
          mode,
          propertyGroup,
          selectedId,
          years,
          [bounds.getWest(), bounds.getSouth(), bounds.getEast(), bounds.getNorth()],
          map.getZoom(),
          controller.signal,
        )
          .then((data) => {
            pointsRef.current = data;
            setVisiblePointCount(data.features.length);
            markersRef.current.forEach((marker) => marker.remove());
            markersRef.current = data.features.map((feature) => {
              const isProperty = feature.properties.featureKind === "property";
              const isBuilding = feature.properties.featureKind === "building";
              const count = Number(feature.properties.transactionCount ?? 1);
              const element = document.createElement("button");
              element.type = "button";
              element.className = `market-map-marker ${mode} ${isProperty || isBuilding ? "property" : "cluster"} ${isBuilding ? "building" : ""}`;
              element.ariaLabel = isBuilding
                ? `Odpri stavbo z ${count} prikazanimi posli`
                : isProperty
                ? `Odpri posel ${feature.properties.transactionId ?? feature.id ?? ""}`
                : `Odpri skupino ${count} poslov`;
              if (isBuilding) element.textContent = count.toLocaleString("sl-SI");
              if (!isProperty && !isBuilding) {
                element.classList.add("geography");
                element.textContent = String(feature.properties.name ?? "");
                const amount = document.createElement("small");
                amount.textContent = amountLabel(feature.properties, mode);
                element.append(amount);
              }
              element.addEventListener("click", (event) => {
                event.stopPropagation();
                const coordinates = feature.geometry.coordinates as [number, number];
                const content = popupContent(feature, mode);
                popupRef.current?.remove();
                const popup = new maplibregl.Popup({ offset: 18, closeButton: false })
                  .setLngLat(coordinates)
                  .setDOMContent(content)
                  .addTo(map);
                popupRef.current = popup;
                popup.on("close", () => {
                  if (popupRef.current === popup) popupRef.current = null;
                });
                if (isBuilding && feature.properties.cadastralMunicipalityCode && feature.properties.buildingNumber) {
                  const buildingController = new AbortController();
                  popup.on("close", () => buildingController.abort());
                  loadBuildingStatistics(
                    Number(feature.properties.cadastralMunicipalityCode),
                    String(feature.properties.buildingNumber),
                    buildingController.signal,
                  ).then((building) => appendBuildingDetail(content, building, map, coordinates))
                    .catch(() => undefined);
                }
                if (isProperty && mode === "sale" && feature.properties.transactionId) {
                  const detailController = new AbortController();
                  popup.on("close", () => detailController.abort());
                  loadSaleTransaction(Number(feature.properties.transactionId), detailController.signal)
                    .then((detail) => appendSaleDetail(content, detail))
                    .catch(() => undefined);
                }
                if (!isProperty && !isBuilding) {
                  zoomToSemanticFeature(map, feature);
                }
              });
              return new maplibregl.Marker({ element, anchor: "center" })
                .setLngLat(feature.geometry.coordinates as [number, number])
                .addTo(map);
            });
            setMapStatus("ready");
          })
          .catch((error: unknown) => {
            if (!(error instanceof DOMException && error.name === "AbortError")) setMapStatus("offline");
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
  }, [mode, propertyGroup, selectedId, years]);

  useEffect(() => {
    if (regions.length === 0) return;
    const controller = new AbortController();
    loadRegionBoundaries(regions, controller.signal).then((data) => {
      boundariesRef.current = data;
      const source = mapRef.current?.getSource("regions") as GeoJSONSource | undefined;
      source?.setData(data);
    }).catch(() => undefined);
    return () => controller.abort();
  }, [regions]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (map.getLayer("region-fill")) {
      map.setPaintProperty(
        "region-fill",
        "fill-opacity",
        selectedId
          ? ["case", ["==", ["get", "id"], selectedId], 0.2, 0.04]
          : 0.045,
      );
    }
    const region = regions.find((item) => item.id === selectedId);
    if (region?.bounds) {
      map.fitBounds(
        [[region.bounds[0], region.bounds[1]], [region.bounds[2], region.bounds[3]]],
        { padding: 45, duration: 700, maxZoom: 11 },
      );
    } else if (!selectedId) {
      map.fitBounds(SLOVENIA_BOUNDS, { padding: 20, duration: 650 });
    }
  }, [selectedId, regions]);

  function changeStyle(next: MapStyleId) {
    setStyleId(next);
    setMapStatus("loading");
    mapRef.current?.setStyle(mapStyle(next));
  }

  return (
    <section className="map-panel real-map-panel" aria-labelledby="map-title">
      <header className="panel-heading map-heading">
        <div>
          <h2 id="map-title">Trg po Sloveniji</h2>
          <p>Premaknite zemljevid ali izberite skupino poslov</p>
        </div>
        <div className="map-key" aria-label="Legenda">
          <span className={`legend-dot ${mode}`} />
          <span>
            {mapStatus === "loading"
              ? "Osvežujem …"
              : mapStatus === "offline"
                ? "Zadnji podatki"
                : `${visiblePointCount.toLocaleString("sl-SI")} skupin na zemljevidu`}
          </span>
        </div>
      </header>
      <div ref={containerRef} className="maplibre-canvas" aria-label="Interaktivni zemljevid nepremičninskih poslov v Sloveniji" />
      <div className="building-search">
        <label>
          <Search size={16} />
          <span className="sr-only">Poišči stavbo po naslovu</span>
          <input
            value={searchQuery}
            onChange={(event) => {
              const next = event.target.value;
              setSearchQuery(next);
              if (next.trim().length < 3) setSearchResults([]);
              setSelectedBuilding(undefined);
            }}
            placeholder="Ulica, hišna številka ali kraj"
            aria-label="Poišči stavbo po naslovu"
          />
          {searchQuery && (
            <button
              type="button"
              aria-label="Počisti iskanje"
              onClick={() => {
                setSearchQuery("");
                setSelectedBuilding(undefined);
              }}
            >
              <X size={15} />
            </button>
          )}
        </label>
        {searchResults.length > 0 && (
          <div className="building-search-results">
            {searchResults.map((result) => (
              <button
                type="button"
                key={`${result.cadastralMunicipalityCode}-${result.buildingNumber}`}
                onClick={() => chooseBuilding(result)}
              >
                <strong>{[result.street, result.houseNumber].filter(Boolean).join(" ")}</strong>
                <span>{result.settlement} · {result.saleCount.toLocaleString("sl-SI")} prodaj</span>
              </button>
            ))}
          </div>
        )}
        {selectedBuilding && (
          <div className="building-result-card">
            <strong>Stavba {selectedBuilding.buildingNumber}</strong>
            <span>
              {selectedBuilding.sales.transactionCount.toLocaleString("sl-SI")} prodaj
              {selectedBuilding.constructionYear ? ` · zgrajena ${selectedBuilding.constructionYear}` : ""}
            </span>
            <div>
              {selectedBuilding.saleTransactions.length === 0
                ? <small>Za stavbo ni prodajnih poslov.</small>
                : selectedBuilding.saleTransactions.map((transaction) => (
                  <button
                    type="button"
                    key={transaction.transactionId}
                    onClick={() => openBuildingTransaction(transaction.transactionId)}
                  >
                    <span>#{transaction.transactionId} · {transaction.contractDate ?? "brez datuma"}</span>
                    <strong>
                      {transaction.amount?.toLocaleString("sl-SI") ?? "—"} €
                      {" · "}
                      {transaction.area?.toLocaleString("sl-SI") ?? "—"} m²
                      {" · "}
                      {transaction.amountPerSqm?.toLocaleString("sl-SI", { maximumFractionDigits: 2 }) ?? "—"} €/m²
                    </strong>
                  </button>
                ))}
            </div>
          </div>
        )}
      </div>
      <div className="map-style-control" role="group" aria-label="Vrsta zemljevida">
        <button type="button" className={styleId === "street" ? "active" : ""} onClick={() => changeStyle("street")}>
          <Layers3 size={15} /> Ceste
        </button>
        <button type="button" className={styleId === "satellite" ? "active" : ""} onClick={() => changeStyle("satellite")}>
          <Mountain size={15} /> Satelit
        </button>
      </div>
      <button
        type="button"
        className="map-home-control"
        aria-label="Prikaži celo Slovenijo"
        onClick={() => {
          onSelect(undefined);
          mapRef.current?.fitBounds(SLOVENIA_BOUNDS, { padding: 20, duration: 650 });
        }}
      >
        <LocateFixed size={17} />
      </button>
    </section>
  );
}
