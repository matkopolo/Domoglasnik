import { expect, it } from "vitest";
import { googleMapsURL, hasPossibleDevelopment, type HistoryAsset } from "./property-history";

it("uses coordinates without an API key and rejects absent/invalid locations", () => {
  expect(googleMapsURL(46.5, 15.5)).toBe("https://www.google.com/maps/search/?api=1&query=46.5%2C15.5");
  for (const [lat, lon] of [[null, 15], [46, undefined], [NaN, 15], [91, 15], [46, 181], [0, 0]])
    expect(googleMapsURL(lat, lon)).toBeNull();
});
const event: HistoryAsset["events"][number] = { transactionId: 1, componentId: 1, contractDate: "2017-01-01", evidence: "exact_identity", propertyGroup: "land", sourceType: 7, share: "1/1", area: 500, agreementPrice: 10000, componentPrice: null, constructionYear: null, componentCount: 1, representative: true, possibleDuplicate: false };
it("requires a later construction year plus an actual recorded parcel reference for possible development", () => {
  const asset: HistoryAsset = { kind: "parcel", cadastralCode: 678, number: "5/1", partNumber: "", pairs: [], truncated: false, events: [event, { ...event, transactionId: 2, contractDate: "2024-01-01", propertyGroup: "house", evidence: "building_geolocation_reference", constructionYear: 2020 }] };
  expect(hasPossibleDevelopment(asset)).toBe(true);
  for (const year of [null, 1980, 2025]) expect(hasPossibleDevelopment({ ...asset, events: [event, { ...asset.events[1], constructionYear: year }] })).toBe(false);
  expect(hasPossibleDevelopment({ ...asset, kind: "building_part" })).toBe(false);
});
