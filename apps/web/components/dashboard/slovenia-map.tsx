"use client";

import { LocateFixed, Minus, Plus } from "lucide-react";
import { useState } from "react";
import type { Cluster, Geography, MarketMode } from "@/lib/types";

const COUNTRY_BOUNDS = { west: 13.3, east: 16.7, south: 45.35, north: 46.95 };
const WIDTH = 840;
const HEIGHT = 400;

function project(longitude: number, latitude: number) {
  return {
    x: ((longitude - COUNTRY_BOUNDS.west) / (COUNTRY_BOUNDS.east - COUNTRY_BOUNDS.west)) * WIDTH,
    y: ((COUNTRY_BOUNDS.north - latitude) / (COUNTRY_BOUNDS.north - COUNTRY_BOUNDS.south)) * HEIGHT,
  };
}

interface SloveniaMapProps {
  mode: MarketMode;
  clusters: Cluster[];
  regions: Geography[];
  selectedId?: number;
  onSelect: (id?: number) => void;
}

export function SloveniaMap({ mode, clusters, regions, selectedId, onSelect }: SloveniaMapProps) {
  const [zoom, setZoom] = useState(1);
  const maxCount = Math.max(...clusters.map((cluster) => cluster.transactionCount), 1);
  return (
    <section className="map-panel" aria-labelledby="map-title">
      <header className="panel-heading map-heading">
        <div>
          <h2 id="map-title">Trg po Sloveniji</h2>
          <p>Izberite regijo za podrobnejši pregled</p>
        </div>
        <div className="map-key" aria-label="Legenda">
          <span className={`legend-dot ${mode}`} />
          <span>Število poslov</span>
        </div>
      </header>
      <div className="map-canvas">
        <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} role="img" aria-label="Analitični zemljevid Slovenije">
          <defs>
            <pattern id="contours" width="58" height="34" patternUnits="userSpaceOnUse">
              <path d="M-8 26 C8 12, 25 13, 38 22 S66 32, 76 16" fill="none" stroke="#9cad9e" strokeWidth=".45" opacity=".35" />
            </pattern>
            <filter id="shadow"><feDropShadow dx="0" dy="2" stdDeviation="2" floodOpacity=".15" /></filter>
          </defs>
          <rect width={WIDTH} height={HEIGHT} fill="url(#contours)" opacity=".55" />
          <g
            className="map-viewport"
            style={{
              transform: `translate(${WIDTH / 2}px, ${HEIGHT / 2}px) scale(${zoom}) translate(${-WIDTH / 2}px, ${-HEIGHT / 2}px)`,
            }}
          >
            <path
              className="country-shape"
              d="M38 205 L70 159 L117 130 L163 95 L231 108 L279 69 L332 92 L375 61 L432 75 L480 50 L534 67 L587 42 L641 70 L700 66 L755 92 L804 126 L789 168 L818 207 L779 241 L742 246 L715 292 L659 287 L614 325 L560 306 L504 339 L448 314 L390 341 L335 311 L283 333 L239 301 L181 314 L154 275 L102 281 L79 241 Z"
              onClick={() => onSelect(undefined)}
            />
            <path className="region-line" d="M143 101 L179 178 L154 275 M231 108 L250 190 L181 314 M332 92 L321 194 L283 333 M432 75 L421 183 L390 341 M534 67 L526 192 L504 339 M641 70 L619 193 L560 306 M755 92 L699 181 L659 287 M79 241 L250 190 L421 183 L619 193 L789 168" />
            {regions.map((region) => {
              if (!region.bounds) return null;
              const center = project((region.bounds[0] + region.bounds[2]) / 2, (region.bounds[1] + region.bounds[3]) / 2);
              return (
                <g
                  key={region.id}
                  className={`region-target ${selectedId === region.id ? "selected" : ""}`}
                  role="button"
                  tabIndex={0}
                  aria-label={`Izberi regijo ${region.name}`}
                  onClick={() => onSelect(region.id)}
                  onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") onSelect(region.id); }}
                >
                  <circle cx={center.x} cy={center.y} r="26" />
                  <text x={center.x} y={center.y + 4}>{region.name.replace(" Slovenija", "")}</text>
                </g>
              );
            })}
            {clusters
              .toSorted((left, right) => right.transactionCount - left.transactionCount)
              .slice(0, 55)
              .map((cluster, index) => {
              const point = project(cluster.longitude, cluster.latitude);
              const radius = 8 + Math.sqrt(cluster.transactionCount / maxCount) * 16;
              return (
                <g key={`${cluster.longitude}-${cluster.latitude}-${index}`} className={`cluster ${mode}`} filter="url(#shadow)">
                  <circle cx={point.x} cy={point.y} r={radius} />
                  <text x={point.x} y={point.y + 4}>{cluster.transactionCount}</text>
                </g>
              );
              })}
          </g>
        </svg>
        <div className="map-controls" aria-label="Kontrole zemljevida">
          <button type="button" aria-label="Približaj" disabled={zoom >= 1.4} onClick={() => setZoom((value) => Math.min(1.4, value + 0.2))}><Plus size={17} /></button>
          <button type="button" aria-label="Oddalji" disabled={zoom <= 1} onClick={() => setZoom((value) => Math.max(1, value - 0.2))}><Minus size={17} /></button>
          <button type="button" aria-label="Prikaži celo Slovenijo" onClick={() => { setZoom(1); onSelect(undefined); }}><LocateFixed size={17} /></button>
        </div>
      </div>
    </section>
  );
}
