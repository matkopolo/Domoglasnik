export function meaningfulText(value: string | null | undefined) {
  const text = value?.trim();
  if (!text || /^(-|n\/a|ni podatka|null)$/i.test(text)) return undefined;
  return text;
}

export function meaningfulNumber(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function meaningfulPosition(value: string | null | undefined) {
  const text = meaningfulText(value);
  if (!text || /^(nadstropje|floor)$/i.test(text)) return undefined;
  return text;
}

export function visibleFields<T>(
  fields: Array<{ label: string; value: T | null | undefined }>,
) {
  return fields.filter((field) => {
    if (typeof field.value === "string") return meaningfulText(field.value) !== undefined;
    return field.value !== null && field.value !== undefined;
  });
}

export const euro = new Intl.NumberFormat("sl-SI", {
  style: "currency",
  currency: "EUR",
  maximumFractionDigits: 0,
});

export const decimal = new Intl.NumberFormat("sl-SI", {
  maximumFractionDigits: 2,
});

export function propertyGroupLabel(value: string) {
  return {
    apartment: "stanovanja",
    house: "hiše",
    land: "zemljišča",
    commercial: "poslovni prostori",
    garage: "garaže",
    room: "sobe",
  }[value] ?? "nepremičnine";
}

export function reliabilityLabel(value: string) {
  return {
    moderate: "srednja",
    low: "nizka",
    very_low: "zelo nizka",
    unavailable: "ni na voljo",
  }[value] ?? value;
}

export function suppressionReasonLabel(
  reason: string | null | undefined,
  sampleSize?: number,
) {
  const count = sampleSize == null
    ? ""
    : ` Trenutno je na voljo ${sampleSize.toLocaleString("sl-SI")} primerljivih poslov.`;
  if (reason === "below_minimum_sample_30") {
    return `Za zanesljiv prikaz potrebujemo najmanj 30 primerljivih poslov.${count}`;
  }
  if (reason === "below_minimum_pair_count_30") {
    return `Za zanesljiv indeks potrebujemo najmanj 30 primerljivih parov.${count}`;
  }
  if (reason === "below_minimum_period_observations_30") {
    return `Za objavo obdobja potrebujemo najmanj 30 opazovanj.${count}`;
  }
  return `Podatek ni objavljen zaradi premajhnega vzorca.${count}`;
}

const anomalyStatuses: Record<string, string> = {
  inlier: "V običajnem razponu primerljivih poslov",
  outlier: "Statistično izstopajoč posel",
  insufficient_cohort: "Premalo primerljivih poslov za oceno odstopanja",
  ineligible: "Posel ni primeren za statistično primerjavo",
};

export function anomalyStatusLabel(status: string | null | undefined) {
  if (!status) return undefined;
  return anomalyStatuses[status] ?? "Statistični status ni določen";
}

const qualityReasons: Record<string, string> = {
  missing_contract_date: "Datum pogodbe ni naveden",
  invalid_contract_date: "Datum pogodbe ni veljaven",
  missing_or_invalid_area: "Površina ni navedena ali ni veljavna",
  missing_or_invalid_price: "Cena ni navedena ali ni veljavna",
  missing_or_invalid_monthly_rent: "Mesečna najemnina ni navedena ali ni veljavna",
  non_comparable_package: "Posel vsebuje mešano ali neprimerljivo vrsto nepremičnin",
  non_market_rent: "Najem ni označen kot tržni najem",
  unknown_vat_rate: "Stopnje DDV ni mogoče zanesljivo določiti",
  invalid_fixed_term_duration: "Trajanje najema za določen čas ni veljavno ali je prekratko",
  implausible_area_for_property_group: "Površina je neobičajna za to vrsto nepremičnine",
  implausible_price_per_sqm: "Cena na kvadratni meter je zunaj dovoljenega razpona",
  implausible_rent_for_property_group: "Najemnina je neobičajna za to vrsto nepremičnine",
  extreme_price_per_m2: "Cena na kvadratni meter izrazito odstopa od primerljivih poslov",
  possible_duplicate: "Posel je morda podvojen",
  missing_coordinates: "Lokacijske koordinate niso na voljo",
  coordinate_outside_slovenia: "Koordinate so zunaj območja Slovenije",
  coordinate_outside_municipality: "Koordinate se ne ujemajo z navedeno občino",
  large_model_residual: "Cena močno odstopa od modelske ocene",
  isolation_forest_anomaly: "Kombinacija lastnosti je statistično neobičajna",
};

export function qualityReasonLabel(reason: string) {
  return qualityReasons[reason] ?? "Podatek ni izpolnil enega od pravil kakovosti";
}
