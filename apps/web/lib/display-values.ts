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

export function reliabilityLabel(value: string) {
  return {
    moderate: "srednja",
    low: "nizka",
    very_low: "zelo nizka",
    unavailable: "ni na voljo",
  }[value] ?? value;
}
