import { describe, expect, it } from "vitest";
import {
  anomalyStatusLabel,
  meaningfulNumber,
  meaningfulPosition,
  meaningfulText,
  qualityReasonLabel,
  suppressionReasonLabel,
  visibleFields,
} from "./display-values";

describe("property display policy", () => {
  it.each([undefined, null, "", "  ", "-", "N/A", "Ni podatka", "null"])(
    "hides missing text %s",
    (value) => expect(meaningfulText(value)).toBeUndefined(),
  );

  it("preserves meaningful zero values", () => {
    expect(meaningfulNumber(0)).toBe(0);
  });

  it("omits a generic position but preserves an actual position", () => {
    expect(meaningfulPosition("nadstropje")).toBeUndefined();
    expect(meaningfulPosition("2. nadstropje")).toBe("2. nadstropje");
  });

  it("filters missing fields without inventing replacements", () => {
    expect(visibleFields<string | number>([
      { label: "A", value: null },
      { label: "B", value: "" },
      { label: "C", value: 0 },
    ])).toEqual([{ label: "C", value: 0 }]);
  });

  it("translates internal suppression codes into useful Slovenian copy", () => {
    expect(suppressionReasonLabel("below_minimum_sample_30", 12)).toBe(
      "Za zanesljiv prikaz potrebujemo najmanj 30 primerljivih poslov. "
      + "Trenutno je na voljo 12 primerljivih poslov.",
    );
    expect(suppressionReasonLabel("internal_unknown", 4)).not.toContain("internal_unknown");
  });

  it.each([
    ["inlier", "V običajnem razponu primerljivih poslov"],
    ["outlier", "Statistično izstopajoč posel"],
    ["insufficient_cohort", "Premalo primerljivih poslov za oceno odstopanja"],
    ["ineligible", "Posel ni primeren za statistično primerjavo"],
  ])("translates anomaly status %s", (status, label) => {
    expect(anomalyStatusLabel(status)).toBe(label);
  });

  it("never exposes internal quality codes", () => {
    expect(qualityReasonLabel("missing_or_invalid_area")).toBe(
      "Površina ni navedena ali ni veljavna",
    );
    expect(qualityReasonLabel("unknown_internal_reason")).not.toContain("unknown_internal_reason");
  });
});
