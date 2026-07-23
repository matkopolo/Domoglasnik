import type { StyleSpecification } from "maplibre-gl";

export type MapStyleId = "street" | "satellite";

export const STREET_STYLE = "https://tiles.openfreemap.org/styles/liberty";

export const SATELLITE_STYLE: StyleSpecification = {
  version: 8,
  glyphs: "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf",
  sources: {
    satellite: {
      type: "raster",
      tiles: [
        "https://tiles.maps.eox.at/wmts/1.0.0/s2cloudless-2020_3857/default/g/{z}/{y}/{x}.jpg",
      ],
      tileSize: 256,
      attribution: "Sentinel-2 cloudless imagery © EOX, contains modified Copernicus Sentinel data",
    },
  },
  layers: [
    {
      id: "satellite",
      type: "raster",
      source: "satellite",
      minzoom: 0,
      maxzoom: 20,
    },
  ],
};

export function mapStyle(style: MapStyleId) {
  return style === "street" ? STREET_STYLE : SATELLITE_STYLE;
}
