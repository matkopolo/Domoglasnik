import { describe, expect, it } from "vitest";
import { meaningfulNumber, meaningfulPosition, meaningfulText, visibleFields } from "./display-values";

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
});
