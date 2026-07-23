import { ArrowDownRight, ArrowUpRight, Building2, MapPin, Sparkles } from "lucide-react";
import type { MarketMode } from "@/lib/types";

export function InsightStrip({ mode }: { mode: MarketMode }) {
  const rent = mode === "rent";
  const insights = [
    { icon: ArrowUpRight, label: "Največja rast", value: rent ? "Obalno-kraška · +9,6 %" : "Podravska · +11,4 %", tone: "green" },
    { icon: ArrowDownRight, label: "Največji padec", value: "Zasavska · −1,2 %", tone: "orange" },
    { icon: Building2, label: "Največ poslov", value: "Osrednjeslovenska", tone: "sage" },
    { icon: Sparkles, label: rent ? "Najvišja najemnina" : "Najvišja mediana", value: rent ? "Ljubljana · 16 €/m²" : "Obalno-kraška · 3.420 €/m²", tone: "sage" },
    { icon: MapPin, label: "Najbolj iskano", value: "Ljubljana, Koper, Maribor", tone: "sage" },
  ];
  return (
    <section className="insight-strip" aria-labelledby="insights-title">
      <h2 id="insights-title">Nedavni vpogledi</h2>
      <div className="insight-list">
        {insights.map(({ icon: Icon, label, value, tone }) => (
          <article key={label}>
            <span className={`insight-icon ${tone}`}><Icon size={19} /></span>
            <div><span>{label}</span><strong>{value}</strong></div>
          </article>
        ))}
      </div>
    </section>
  );
}
