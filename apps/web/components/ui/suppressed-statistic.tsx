import { EyeOff } from "lucide-react";
import { suppressionReasonLabel } from "@/lib/display-values";

export function SuppressedStatistic({ reason, sampleSize, minimum = 30 }: {
  reason?: string | null;
  sampleSize?: number;
  minimum?: number;
}) {
  return <div className="suppressed-statistic" role="status">
    <EyeOff aria-hidden="true" />
    <div><strong>Premalo podatkov</strong>
      <p>{sampleSize != null
        ? `Za objavo potrebujemo najmanj ${minimum} primerljivih poslov. Trenutno jih je ${sampleSize}.`
        : suppressionReasonLabel(reason)}</p>
    </div>
  </div>;
}
