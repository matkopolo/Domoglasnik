-- Public ETN exports contain no party identity columns. Free-text source notes
-- are nevertheless unnecessary for analytics and could contain identifying
-- text entered by a reporter. Do not retain them.
ALTER TABLE transactions DROP COLUMN IF EXISTS notes;
ALTER TABLE building_parts DROP COLUMN IF EXISTS property_notes;
ALTER TABLE land_parcels DROP COLUMN IF EXISTS property_notes;
ALTER TABLE rental_transactions DROP COLUMN IF EXISTS notes;
ALTER TABLE rental_building_parts DROP COLUMN IF EXISTS rented_spaces_notes;
