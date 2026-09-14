import { ExternalLink } from "lucide-react";
import { googleMapsURL } from "@/lib/property-history";

export function LocationLink({ latitude, longitude }: { latitude?: number | null; longitude?: number | null }) {
  const url = googleMapsURL(latitude, longitude);
  if (!url) return null;
  return <div className="context-note"><a href={url} target="_blank" rel="noopener noreferrer">
    Odpri lokacijo v Google Zemljevidih <ExternalLink size={14} aria-hidden="true"/>
  </a><small> Približna lokacija ETN; pri paketu lahko predstavlja središče več sestavin. Odpre zunanjo storitev Google.</small></div>;
}
