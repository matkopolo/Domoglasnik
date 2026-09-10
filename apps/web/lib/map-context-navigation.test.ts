import { afterEach, expect, it, vi } from "vitest";
import { focusAreaContext } from "./map-context-navigation";
import type { Map as LibreMap } from "maplibre-gl";

afterEach(() => vi.unstubAllGlobals());
const years = { from: 2025, to: 2025 };
function camera() {
  return { fitBounds: vi.fn(), easeTo: vi.fn() } as unknown as Pick<LibreMap, "fitBounds" | "easeTo">;
}

it("zooms back out to the actual municipality bounds from a property", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ bounds: [15.4, 46.4, 15.6, 46.6] }))));
  const map = camera();
  await focusAreaContext(map, { kind: "geography", id: 77, level: "municipality", name: "Ruše" }, "apartment", years, new AbortController().signal);
  expect(map.fitBounds).toHaveBeenCalledWith([[15.4, 46.4], [15.6, 46.6]], expect.objectContaining({ maxZoom: 13.4 }));
  expect(map.easeTo).not.toHaveBeenCalled();
});

it("zooms in to a building selected outside the map", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ longitude: 15.5, latitude: 46.5 }))));
  const map = camera();
  await focusAreaContext(map, { kind: "building", cadastralCode: 665, buildingNumber: "8" }, "apartment", years, new AbortController().signal);
  expect(map.easeTo).toHaveBeenCalledWith(expect.objectContaining({ center: [15.5, 46.5], zoom: 17 }));
});

it("does not move the camera for stale geography results", async () => {
  const controller = new AbortController();
  vi.stubGlobal("fetch", vi.fn(async () => { controller.abort(); return new Response(JSON.stringify({ bounds: [15.4, 46.4, 15.6, 46.6] })); }));
  const map = camera();
  await expect(focusAreaContext(map, { kind: "geography", id: 77, level: "municipality", name: "Ruše" }, "apartment", years, controller.signal)).rejects.toThrow();
  expect(map.fitBounds).not.toHaveBeenCalled();
});

it("keeps the map in place when source coordinates are missing", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({}))));
  const map = camera();
  expect(await focusAreaContext(map, { kind: "building", cadastralCode: 665, buildingNumber: "8" }, "apartment", years, new AbortController().signal)).toContain("nima uporabnih koordinat");
  expect(map.easeTo).not.toHaveBeenCalled();
});
