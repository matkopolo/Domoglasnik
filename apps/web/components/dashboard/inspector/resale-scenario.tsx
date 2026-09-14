"use client";

import { useState, type FormEvent } from "react";
import { calculateResaleScenario, TAX_RULES, type ScenarioInput } from "@/lib/resale-scenario";
import { decimal, euro } from "@/lib/display-values";
import styles from "./property-history.module.css";

export function ResaleScenario() {
  const [result, setResult] = useState<ReturnType<typeof calculateResaleScenario>>();
  const [error, setError] = useState<string>();
  function calculate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const number = (key: string) => data.get(key) === "" ? NaN : Number(data.get(key));
    try {
      setResult(calculateResaleScenario({
        acquired: String(data.get("acquired")), disposed: String(data.get("disposed")),
        purchase: number("purchase"), sale: number("sale"), cashCosts: number("cashCosts"), taxableGain: number("taxableGain"),
        transferTax: String(data.get("transferTax")) as ScenarioInput["transferTax"],
        mainHomeExemption: data.get("mainHomeExemption") === "on", confirmed: data.get("confirmed") === "on",
      })); setError(undefined);
    } catch (problem) { setResult(undefined); setError(problem instanceof Error ? problem.message : "Preverite vnos."); }
  }
  return <form className={styles.scenario} onSubmit={calculate} onChange={() => { setResult(undefined); setError(undefined); }}>
    <p>Izobraževalni scenarij za fizično osebo, zasebno prodajo v 2026 in brez DDV. Ni davčna napoved, pravni nasvet ali ocena dobička prejšnjega prodajalca.
      Podatkov ne shranjujemo; izračun ostane v brskalniku.</p>
    <p>Davek na obdavčljivi dobiček: manj kot 5 dopolnjenih let 25 %, 5–9 let 20 %, 10–14 let 15 %, od 15 let 0 %.
      DPN praviloma 2 %; izberite dejanskega plačnika oziroma potrjeno oprostitev.</p>
    <div className={styles.metadata}>
      <label>Datum pridobitve<input type="date" name="acquired" min="2002-01-01" required/></label>
      <label>Datum prodaje (2026)<input type="date" name="disposed" min="2026-01-01" max="2026-12-31" required/></label>
      <label>Nakupna cena (€)<input type="number" name="purchase" min="0.01" max="10000000000" step="0.01" required/></label>
      <label>Prodajna cena (€)<input type="number" name="sale" min="0.01" max="10000000000" step="0.01" required/></label>
    </div>
    <label>Drugi denarni stroški skupaj (€)<input type="number" name="cashCosts" min="0" max="10000000000" step="0.01" required/></label>
    <small>Vključite nakupne/prodajne stroške, obnovo, gradnjo, obresti in podobno. Brez nakupne cene in davkov ob tej prodaji, ki jih izračunamo spodaj. Ne vštevajte odplačila glavnice posojila še enkrat.</small>
    <label>Potrjena davčna osnova po priznanih odbitkih (€)<input type="number" name="taxableGain" min="0" max="10000000000" step="0.01" required/></label>
    <small>Ni samodejno enaka razliki cen. Upoštevati je treba priznane normirane stroške, ustrezne plačane davke in dokazljiva vlaganja. Denarni stroški niso vsi davčno priznani. Osnovo preverite s strokovnjakom.</small>
    <label>Plačilo DPN<select name="transferTax" required defaultValue=""><option value="" disabled>Izberite predpostavko</option><option value="seller">Prodajalec · 2 % prodajne cene</option><option value="buyer">Kupec po dogovoru · prodajalec 0 €</option><option value="exempt">Potrjena oprostitev DPN · 0 €</option></select></label>
    <label><input type="checkbox" name="mainHomeExemption"/>Predpostavljam potrjeno oprostitev za lastni dom.</label>
    <small>Stanovanje ali hiša z največ dvema stanovanjema: lastništvo, stalno prebivališče in dejansko bivanje zadnja tri leta ter izpolnjeni drugi zakonski pogoji. Poslovna uporaba/oddajanje zahtevata posebno preveritev.</small>
    <label><input type="checkbox" name="confirmed" required/>Potrjujem zasebno prodajo brez DDV, predpostavke in preverjeno davčno osnovo; prodajna cena ustreza osnovi za DPN. To ni davčni obračun.</label>
    <button type="submit">Izračunaj scenarij</button>
    {error && <p role="alert">{error}</p>}
    {result && <output aria-live="polite"><dl className={styles.metadata}>
      <div><dt>Dopolnjena leta</dt><dd>{result.completedYears}</dd></div>
      <div><dt>Stopnja na davčno osnovo</dt><dd>{decimal.format(result.capitalGainsRate * 100)} %</dd></div>
      <div><dt>Davek na dobiček v scenariju</dt><dd>{euro.format(result.capitalGainsTax)}</dd></div>
      <div><dt>DPN prodajalca</dt><dd>{euro.format(result.transferTax)}</dd></div>
      <div><dt>Prodajni prejemek po vnesenih stroških in davkih</dt><dd>{euro.format(result.netProceeds)}</dd></div>
      <div><dt>Razlika po nakupni ceni, stroških in davkih</dt><dd>{euro.format(result.netDifference)}</dd></div>
    </dl><small>Nominalno, brez inflacijske prilagoditve; ne vključuje nevnesenih stroškov. {result.ruleVersion}</small></output>}
    <small><a href={TAX_RULES.source} target="_blank" rel="noopener noreferrer">FURS: pogoji, davčne osnove in oprostitve</a> · pravila preverjena 10. 9. 2026. Pred komercialno uporabo je potreben strokovni davčni pregled.</small>
  </form>;
}
