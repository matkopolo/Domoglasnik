import { MapPin } from "lucide-react";

export function Brand() {
  return (
    <div className="brand" aria-label="Analitika nepremičnin">
      <span className="brand-mark" aria-hidden="true">
        <MapPin size={19} strokeWidth={2.2} />
      </span>
      <span>Analitika nepremičnin</span>
    </div>
  );
}
