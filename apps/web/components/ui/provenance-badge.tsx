import { Calculator, Database, Sigma } from "lucide-react";

export type ProvenanceKind = "source" | "calculated" | "model";

const labels: Record<ProvenanceKind, string> = {
  source: "Podatek GURS ETN",
  calculated: "Izračun aplikacije",
  model: "Statistična modelska ocena",
};

export function ProvenanceBadge({ kind }: { kind: ProvenanceKind }) {
  const Icon = kind === "source" ? Database : kind === "calculated" ? Calculator : Sigma;
  return <span className={`provenance-badge provenance-${kind}`}><Icon aria-hidden="true" />{labels[kind]}</span>;
}
