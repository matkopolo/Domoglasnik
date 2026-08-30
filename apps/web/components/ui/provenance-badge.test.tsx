import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProvenanceBadge } from "./provenance-badge";

describe("ProvenanceBadge", () => {
  it("distinguishes source, calculated and model evidence", () => {
    const { rerender } = render(<ProvenanceBadge kind="source"/>);
    expect(screen.getByText("Podatek GURS ETN")).toHaveClass("provenance-source");
    rerender(<ProvenanceBadge kind="calculated"/>);
    expect(screen.getByText("Izračun aplikacije")).toHaveClass("provenance-calculated");
    rerender(<ProvenanceBadge kind="model"/>);
    expect(screen.getByText("Statistična modelska ocena")).toHaveClass("provenance-model");
  });
});
