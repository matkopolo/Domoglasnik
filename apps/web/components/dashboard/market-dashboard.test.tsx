import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MarketDashboard } from "./market-dashboard";

vi.mock("@/lib/api", async () => {
  const { demoDashboard } = await import("@/lib/demo-data");
  return { loadDashboard: vi.fn(async (mode: "sale" | "rent") => demoDashboard(mode)) };
});

afterEach(cleanup);

describe("MarketDashboard", () => {
  it("renders the map-first market overview", async () => {
    render(<MarketDashboard />);
    expect(screen.getByRole("heading", { name: "Pregled trga" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Analitični zemljevid Slovenije" })).toBeInTheDocument();
    const metric = await screen.findByText("Srednja cena / m²");
    expect(metric.nextElementSibling).toHaveTextContent("2596");
  });

  it("switches to rental metrics", async () => {
    render(<MarketDashboard />);
    fireEvent.click(screen.getByRole("button", { name: "Najem" }));
    const metric = await screen.findByText("Srednja najemnina / m²");
    expect(metric.nextElementSibling).toHaveTextContent("12");
  });
});
