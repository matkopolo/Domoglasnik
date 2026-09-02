import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DataPublicationStatus } from "./data-publication-status";

describe("DataPublicationStatus", () => {
  it("explains that the active release remains visible during processing", () => {
    render(<DataPublicationStatus mode="sale" status={{
      status: "processing",
      activeRelease: {
        id: "old", status: "published", stage: "published",
        startedAt: "2026-04-01T08:00:00Z", analyticsSaleThrough: "2026-03-31",
      },
      pendingRelease: {
        id: "new", status: "processing", stage: "models",
        startedAt: "2026-07-01T08:00:00Z", sourceSaleThrough: "2026-06-30",
      },
    }} />);

    expect(screen.getByText("Novi podatki GURS so v obdelavi")).toBeInTheDocument();
    expect(screen.getByText(/zadnjo zaključeno analitiko do 31\. 3\. 2026/)).toBeInTheDocument();
    expect(screen.getByText(/učenje in preverjanje modelov/)).toBeInTheDocument();
  });

  it("reports the published rental cutoff", () => {
    render(<DataPublicationStatus mode="rent" status={{
      status: "current",
      activeRelease: {
        id: "active", status: "published", stage: "published",
        startedAt: "2026-04-01T08:00:00Z", analyticsRentThrough: "2025-12-31",
      },
    }} />);
    expect(screen.getByText(/31\. 12\. 2025/)).toBeInTheDocument();
  });
});
