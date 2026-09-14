import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { ResaleScenario } from "./resale-scenario";
afterEach(cleanup);
it("calculates explicit assumptions and clears the result when an input changes", () => {
  render(<ResaleScenario/>);
  for (const [label, value] of [["Datum pridobitve", "2021-01-01"], ["Datum prodaje (2026)", "2026-01-01"], ["Nakupna cena (€)", "100000"], ["Prodajna cena (€)", "150000"], ["Drugi denarni stroški skupaj (€)", "10000"], ["Potrjena davčna osnova po priznanih odbitkih (€)", "35000"], ["Plačilo DPN", "seller"]])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  fireEvent.click(screen.getByLabelText(/Potrjujem zasebno prodajo/));
  fireEvent.click(screen.getByRole("button", { name: "Izračunaj scenarij" }));
  expect(screen.getByText("30.000 €")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Prodajna cena (€)"), { target: { value: "160000" } });
  expect(screen.queryByText("30.000 €")).not.toBeInTheDocument();
});
