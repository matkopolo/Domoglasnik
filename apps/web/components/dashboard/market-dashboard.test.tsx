import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MarketDashboard } from "./market-dashboard";

vi.mock("@/lib/api", async () => {
  const { demoDashboard } = await import("@/lib/demo-data");
  return { loadDashboard: vi.fn(async (mode: "sale" | "rent") => demoDashboard(mode)) };
});

vi.mock("./slovenia-map", () => ({
  SloveniaMap: () => <div role="application" aria-label="Interaktivni zemljevid nepremičninskih poslov v Sloveniji" />,
}));

afterEach(cleanup);

describe("MarketDashboard", () => {
  it("renders the map-first market overview", async () => {
    render(<MarketDashboard />);
    expect(screen.getByRole("heading", { name: "Pregled trga" })).toBeInTheDocument();
    expect(screen.getByRole("application", { name: "Interaktivni zemljevid nepremičninskih poslov v Sloveniji" })).toBeInTheDocument();
    const metric = await screen.findByText("Srednja cena / m²");
    expect(metric.nextElementSibling).toHaveTextContent("2596");
  });

  it("switches to rental metrics", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    const metric = await screen.findByText("Srednja najemnina / m²");
    expect(metric.nextElementSibling).toHaveTextContent("12");
  });

  it("allows the analysis year range to be changed", () => {
    render(<MarketDashboard />);
    const from = screen.getByRole("combobox", { name: "Od leta" });
    const to = screen.getByRole("combobox", { name: "Do leta" });
    fireEvent.change(from, { target: { value: "2020" } });
    fireEvent.change(to, { target: { value: "2024" } });
    expect(from).toHaveValue("2020");
    expect(to).toHaveValue("2024");
  });
});
