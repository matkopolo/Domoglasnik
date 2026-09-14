// Deliberately narrow, versioned educational scenario. Never applied automatically to
// historic ETN transactions. Taxable gain is supplied/confirmed separately from cash costs.
export const TAX_RULES = {
  id: "SI-private-sale-2026-reviewed-2026-09-10",
  disposalFrom: "2026-01-01", disposalThrough: "2026-12-31",
  source: "https://www.fu.gov.si/zivljenjski_dogodki_prebivalci/nakup_in_prodaja_nepremicnine",
} as const;

export interface ScenarioInput {
  acquired: string; disposed: string;
  purchase: number; sale: number; cashCosts: number; taxableGain: number;
  transferTax: "seller" | "buyer" | "exempt";
  mainHomeExemption: boolean;
  confirmed: boolean;
}

function validDate(value: string) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) && Number.isFinite(Date.parse(value))
    && new Date(value).toISOString().slice(0, 10) === value;
}

export function calculateResaleScenario(input: ScenarioInput) {
  if (!validDate(input.acquired) || !validDate(input.disposed) || input.acquired >= input.disposed)
    throw new Error("Vnesite veljavna datuma; pridobitev mora biti pred prodajo.");
  if (input.disposed < TAX_RULES.disposalFrom || input.disposed > TAX_RULES.disposalThrough)
    throw new Error("Ta različica scenarija podpira le prodaje v letu 2026; zgodovinska in prihodnja pravila niso preverjena.");
  if (input.acquired < "2002-01-01")
    throw new Error("Pridobitve pred letom 2002 zahtevajo ločeno davčno obravnavo; scenarij jih ne podpira.");
  if (![input.purchase, input.sale, input.cashCosts, input.taxableGain].every((v) => Number.isFinite(v) && v >= 0 && v <= 1e10)
    || input.purchase <= 0 || input.sale <= 0)
    throw new Error("Ceni morata biti pozitivni, stroški in davčna osnova pa veljavni nenegativni zneski.");
  if (!input.confirmed) throw new Error("Najprej potrdite predpostavke in davčno osnovo.");
  if (!["seller", "buyer", "exempt"].includes(input.transferTax)) throw new Error("Izberite obravnavo DPN.");
  let years = Number(input.disposed.slice(0, 4)) - Number(input.acquired.slice(0, 4));
  if (input.disposed.slice(5) < input.acquired.slice(5)) years--;
  if (input.mainHomeExemption && years < 3) throw new Error("Za navedeno oprostitev je potrebno najmanj triletno lastništvo in izpolnjevanje drugih pogojev.");
  const capitalGainsRate = input.mainHomeExemption || years >= 15 ? 0 : years >= 10 ? 0.15 : years >= 5 ? 0.20 : 0.25;
  const capitalGainsTax = Math.round(input.taxableGain * capitalGainsRate * 100) / 100;
  const transferTax = input.transferTax === "seller" ? Math.round(input.sale * 0.02 * 100) / 100 : 0;
  return {
    ruleVersion: TAX_RULES.id, completedYears: years, capitalGainsRate, capitalGainsTax, transferTax,
    netProceeds: input.sale - input.cashCosts - transferTax - capitalGainsTax,
    netDifference: input.sale - input.purchase - input.cashCosts - transferTax - capitalGainsTax,
  };
}
