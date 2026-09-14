import { expect, it } from "vitest";
import { calculateResaleScenario, type ScenarioInput } from "./resale-scenario";

const base: ScenarioInput = { acquired: "2021-09-11", disposed: "2026-09-11", purchase: 100000, sale: 150000,
  cashCosts: 10000, taxableGain: 35000, transferTax: "seller", mainHomeExemption: false, confirmed: true };
it.each([
  ["2021-09-12", .25], ["2021-09-11", .20], ["2016-09-12", .20],
  ["2016-09-11", .15], ["2011-09-12", .15], ["2011-09-11", 0],
])("uses completed anniversaries for %s", (acquired, rate) => {
  expect(calculateResaleScenario({ ...base, acquired }).capitalGainsRate).toBe(rate);
});
it("separates taxable gain from cash costs and charges transfer tax once", () => {
  expect(calculateResaleScenario(base)).toMatchObject({ transferTax: 3000, capitalGainsTax: 7000, netProceeds: 130000, netDifference: 30000 });
  expect(calculateResaleScenario({ ...base, transferTax: "buyer" }).transferTax).toBe(0);
  expect(calculateResaleScenario({ ...base, taxableGain: 0 }).capitalGainsTax).toBe(0);
});
it("requires explicit assumptions and refuses unverified historical/future dates", () => {
  for (const update of [{ confirmed: false }, { disposed: "2025-09-11" }, { disposed: "2027-01-01" },
    { acquired: "2026-02-30" }, { cashCosts: NaN }, { purchase: -1 }, { acquired: "2001-01-01" },
    { acquired: "2026-01-01", mainHomeExemption: true }]) {
    expect(() => calculateResaleScenario({ ...base, ...update })).toThrow();
  }
  expect(calculateResaleScenario({ ...base, mainHomeExemption: true }).capitalGainsTax).toBe(0);
});
