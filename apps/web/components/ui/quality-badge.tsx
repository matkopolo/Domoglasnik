import { AlertTriangle, CheckCircle2, Info } from "lucide-react";

export function QualityBadge({ representative, label }: { representative?: boolean; label?: string }) {
  const Icon = representative === true ? CheckCircle2 : representative === false ? AlertTriangle : Info;
  const text = label ?? (representative === true
    ? "Reprezentativen posel"
    : representative === false ? "Posel ni v reprezentativnem vzorcu" : "Status ni na voljo");
  return <span className={`quality-badge quality-${representative === true ? "representative" : representative === false ? "caution" : "neutral"}`}>
    <Icon aria-hidden="true" />{text}
  </span>;
}
