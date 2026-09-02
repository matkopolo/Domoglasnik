import { AlertTriangle, Database, RefreshCw } from "lucide-react";
import type { DataStatus, MarketMode } from "@/lib/types";

function formattedDate(value?: string) {
  if (!value) return "ni navedeno";
  return new Intl.DateTimeFormat("sl-SI", {
    day: "numeric", month: "numeric", year: "numeric",
  }).format(new Date(`${value.slice(0, 10)}T00:00:00`));
}

function analyticsCutoff(status: DataStatus["activeRelease"], mode: MarketMode) {
  return mode === "sale" ? status?.analyticsSaleThrough : status?.analyticsRentThrough;
}

function sourceCutoff(status: DataStatus["pendingRelease"], mode: MarketMode) {
  return mode === "sale" ? status?.sourceSaleThrough : status?.sourceRentThrough;
}

const stages: Record<string, string> = {
  source: "preverjanje in uvoz vira GURS",
  geography: "usklajevanje geografskih meja",
  analytics: "preračun analitike",
  models: "učenje in preverjanje modelov",
  published: "objava",
};

export function DataPublicationStatus({ status, mode }: { status?: DataStatus; mode: MarketMode }) {
  if (!status) return null;
  const activeThrough = analyticsCutoff(status.activeRelease, mode);
  if (status.pendingRelease) {
    const sourceStage = status.pendingRelease.stage === "source";
    return <div className="publication-status processing" role="status">
      <RefreshCw aria-hidden="true" className="spin" />
      <div><strong>{sourceStage ? "Preverjamo nove podatke GURS" : "Novi podatki GURS so v obdelavi"}</strong>
        <span>Prikazujemo zadnjo zaključeno analitiko do {formattedDate(activeThrough)}. {sourceStage ? "Preverjamo, ali je GURS objavil novejši vir." : <>Novi vir vsebuje podatke do {formattedDate(sourceCutoff(status.pendingRelease, mode))}; trenutno poteka {stages[status.pendingRelease.stage] ?? status.pendingRelease.stage}.</>}</span>
      </div>
    </div>;
  }
  if (status.status === "failed" && status.lastFailedRelease) {
    return <div className="publication-status failed" role="status">
      <AlertTriangle aria-hidden="true" />
      <div><strong>Zadnja posodobitev ni uspela</strong>
        <span>Še vedno prikazujemo zadnjo preverjeno analitiko do {formattedDate(activeThrough)}.</span>
      </div>
    </div>;
  }
  return <div className="publication-status current compact" role="status">
    <Database aria-hidden="true" />
    <span>Objavljena analitika GURS ETN do {formattedDate(activeThrough)}</span>
  </div>;
}
