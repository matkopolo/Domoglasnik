import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useState } from "react";
import { FilterBar } from "./filter-bar";
import type { MarketConstraints } from "@/lib/types";

afterEach(cleanup);

function LandFilters() {
  const [area, setArea] = useState<MarketConstraints>({ min: 60, max: 100 });
  return <FilterBar mode="sale" propertyGroup="land" area={area} onAreaChange={setArea}
    years={{ from: 2024, to: 2025 }} geographies={[]} onModeChange={vi.fn()}
    onPropertyChange={vi.fn()} onGeographyChange={vi.fn()} onYearsChange={vi.fn()}/>;
}

it("places land scope outside the crowded date/area row with short labels and an accessible explanation", () => {
  render(<LandFilters/>);
  const select = screen.getByRole("combobox", { name: "Vrste zemljišč" });
  expect(select.closest(".secondary-filter-group")).toBeNull();
  expect(select.closest(".land-filter-row")).not.toBeNull();
  expect(within(select).getAllByRole("option").map((option) => option.textContent)).toEqual([
    "Vsa zemljišča", "Zazidljiva zemljišča", "Stavbna zemljišča · ETN 3",
    "Kmetijska zemljišča", "Gozdna zemljišča", "Mešana zemljišča", "Druga / neznana namembnost",
  ]);
  expect(select).toHaveAccessibleDescription(/samo preverjeni kodi ETN 1 in 2/);
  fireEvent.change(select, { target: { value: "all" } });
  expect(select).toHaveValue("all");
  expect(select).toHaveAccessibleDescription(/tudi kmetijska, gozdna/);
  expect(screen.getByRole("spinbutton", { name: "Površina od" })).toHaveValue(60);
  expect(screen.getByRole("spinbutton", { name: "Površina do" })).toHaveValue(100);
  fireEvent.change(select, { target: { value: "agricultural" } });
  expect(select).toHaveAccessibleDescription(/ETN 7/);
  expect(select).toHaveValue("agricultural");
  fireEvent.change(select, { target: { value: "forest" } });
  expect(select).toHaveAccessibleDescription(/ETN 9/);
  fireEvent.change(select, { target: { value: "mixed" } });
  expect(select).toHaveAccessibleDescription(/Celotne pogodbene cene ne pripisujemo/);
});

it("keeps land selection available inside the mobile filter panel", () => {
  render(<LandFilters/>);
  fireEvent.click(screen.getByRole("button", { name: "Spremeni filtre" }));
  const panel = screen.getByRole("dialog", { name: "Spremeni filtre" });
  const select = within(panel).getByRole("combobox", { name: "Vrste zemljišč" });
  select.focus();
  expect(select).toHaveFocus();
  fireEvent.change(select, { target: { value: "all" } });
  expect(select).toHaveValue("all");
  fireEvent.keyDown(select, { key: "Escape" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});
