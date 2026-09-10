import { describe, expect, it } from "vitest";
import { isLandScope, landScopeOptions, landScopes } from "./land-scopes";
import { marketQuery } from "./api";

describe("land categories", () => {
  it.each(landScopeOptions)("passes %s unchanged to filtered API queries", (scope) => {
    expect(isLandScope(scope)).toBe(true);
    expect(marketQuery("land", { from: 2024, to: 2025 }, 77, { landScope: scope, min: 100 }).get("landScope")).toBe(scope);
    expect(landScopes[scope].description.length).toBeGreaterThan(20);
  });
  it("preserves old broad URLs without offering broad as a new category", () => {
    expect(isLandScope("broad")).toBe(true);
    expect(landScopeOptions).not.toContain("broad");
    expect(isLandScope("speculative")).toBe(false);
    expect(isLandScope("toString")).toBe(false);
    expect(isLandScope(null)).toBe(false);
  });
});
