import type { LandScope } from "./types";

export const landScopes: Record<LandScope, { label: string; description: string }> = {
  all: { label: "Vsa zemljišča", description: "Vsa zemljišča: tudi kmetijska, gozdna, mešana in neznane namembnosti. Skupna mediana ni ocena posamezne vrste zemljišča." },
  strict: { label: "Zazidljiva zemljišča", description: "Strogi vzorec zazidljivih zemljišč: samo preverjeni kodi ETN 1 in 2 na vseh parcelah posla." },
  unverified: { label: "Stavbna zemljišča · ETN 3", description: "Samo ETN 3: delno komunalno opremljena ali neopremljena zemljišča, kjer je ali bo mogoča gradnja. Niso vključena v strogi zazidljivi model." },
  agricultural: { label: "Kmetijska zemljišča", description: "Samo kmetijske parcele: ETN 7 (njive, travniki, pašniki) in 8 (trajni nasadi). Posli z gozdnimi ali drugimi vrstami parcel so prikazani ločeno med mešanimi." },
  forest: { label: "Gozdna zemljišča", description: "Samo gozdne parcele: ETN 9. Posli, ki vključujejo tudi kmetijske ali druge vrste parcel, so prikazani ločeno med mešanimi." },
  mixed: { label: "Mešana zemljišča", description: "En posel vključuje več kategorij zemljišč, na primer kmetijska in gozdna ali znano in neznano namembnost. Celotne pogodbene cene ne pripisujemo posamezni kategoriji." },
  other: { label: "Druga / neznana namembnost", description: "Samo druge ali neznane vrste: ETN 4, 5, 6 in 10 (infrastruktura, pripadajoča, neplodna ali vodna zemljišča), manjkajoče ali neprepoznane kode. Vključeni so tudi posli brez zemljiške klasifikacije." },
  broad: { label: "Širši stavbni trg (prejšnji filter)", description: "Prejšnji filter iz shranjene povezave: kode ETN 1, 2 in 3 skupaj. Za ločen prikaz izberite zazidljiva zemljišča ali stavbna zemljišča ETN 3." },
};
export const landScopeOptions: LandScope[] = ["all", "strict", "unverified", "agricultural", "forest", "mixed", "other"];
export function isLandScope(value: string | null): value is LandScope {
  return value != null && Object.hasOwn(landScopes, value);
}
