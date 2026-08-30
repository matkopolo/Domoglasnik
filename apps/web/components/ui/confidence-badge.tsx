import { Gauge } from "lucide-react";
import { reliabilityLabel } from "@/lib/display-values";

export function ConfidenceBadge({ confidence }: { confidence?: string | null }) {
  return <span className={`confidence-badge confidence-${confidence ?? "unknown"}`}>
    <Gauge aria-hidden="true" />Zanesljivost: {reliabilityLabel(confidence ?? "unavailable")}
  </span>;
}
