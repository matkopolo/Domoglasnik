"use client";

import { Menu, RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { loadDashboard } from "@/lib/api";
import { demoDashboard } from "@/lib/demo-data";
import type { DashboardData, MarketMode, PropertyGroup, YearRange } from "@/lib/types";
import { Brand } from "@/components/ui/brand";
import { DistributionChart } from "./distribution-chart";
import { FilterBar } from "./filter-bar";
import { InsightStrip } from "./insight-strip";
import { SloveniaMap } from "./slovenia-map";
import { SummaryPanel } from "./summary-panel";
import { TrendChart } from "./trend-chart";

export function MarketDashboard() {
  const [mode, setMode] = useState<MarketMode>("sale");
  const [propertyGroup, setPropertyGroup] = useState<PropertyGroup>("apartment");
  const [geographyId, setGeographyId] = useState<number>();
  const [years, setYears] = useState<YearRange>({ from: 2018, to: 2025 });
  const [data, setData] = useState<DashboardData>(() => demoDashboard("sale"));
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();
    loadDashboard(mode, propertyGroup, geographyId, years, controller.signal)
      .then(setData)
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [mode, propertyGroup, geographyId, years]);

  const selectedName = useMemo(
    () => data.geographies.find((region) => region.id === geographyId)?.name ?? "Slovenija",
    [data.geographies, geographyId],
  );

  return (
    <div className={`app-shell ${loading ? "is-loading" : ""}`}>
      <header className="topbar">
        <button className="mobile-menu" aria-label="Odpri navigacijo"><Menu /></button>
        <Brand />
        <nav aria-label="Glavna navigacija">
          <a href="#overview" className="active">Pregled trga</a>
          <a href="#trends">Gibanje</a>
          <a href="#distribution">Porazdelitev</a>
          <a href="#insights">Vpogledi</a>
        </nav>
        <div className="updated">
          <RefreshCw size={14} className={loading ? "spin" : ""} />
          <span>Posodobljeno: 23. 7. 2026</span>
          <i className={data.source} />
          <small>{data.source === "live" ? "API" : "Predstavitveni podatki"}</small>
        </div>
      </header>
      <main id="overview">
        <div className="page-intro">
          <div><h1>Pregled trga</h1><p>Preverjeni podatki o prodajah in najemih nepremičnin v Sloveniji.</p></div>
        </div>
        <FilterBar
          mode={mode}
          onModeChange={(next) => {
            setLoading(true);
            setMode(next);
            if (next === "rent" && propertyGroup === "land") setPropertyGroup("apartment");
          }}
          propertyGroup={propertyGroup}
          onPropertyChange={(next) => { setLoading(true); setPropertyGroup(next); }}
          geographyId={geographyId}
          onGeographyChange={(next) => { setLoading(true); setGeographyId(next); }}
          geographies={data.geographies}
          years={years}
          onYearsChange={(next) => { setLoading(true); setYears(next); }}
        />
        <div className="primary-grid">
          <SloveniaMap
            mode={mode}
            propertyGroup={propertyGroup}
            clusters={data.clusters}
            regions={data.geographies}
            selectedId={geographyId}
            years={years}
            onSelect={(next) => { setLoading(true); setGeographyId(next); }}
          />
          <SummaryPanel title={selectedName} mode={mode} overview={data.overview} series={data.timeSeries} />
        </div>
        <div className="chart-grid" id="trends">
          <TrendChart mode={mode} points={data.timeSeries} years={years} />
          <DistributionChart mode={mode} data={data.distribution} />
        </div>
        <div id="insights"><InsightStrip mode={mode} /></div>
        <footer>
          <span>Vir: GURS ETN in RPE · Reprezentativne statistike ne vključujejo potrjenih odstopanj.</span>
          <a href="#overview">Metodologija</a>
        </footer>
      </main>
    </div>
  );
}
