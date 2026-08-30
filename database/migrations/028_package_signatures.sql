CREATE OR REPLACE FUNCTION analytics.sale_package_signature(subject_id bigint)
RETURNS text
LANGUAGE sql
STABLE
AS $$
SELECT concat(
  'building[',coalesce((
    SELECT string_agg(coalesce(part.building_part_type::text,'?'),','
                      ORDER BY part.building_part_type NULLS FIRST,part.id)
    FROM building_parts part
    WHERE part.transaction_id=subject_id
  ),''),'];land[',coalesce((
    SELECT string_agg(coalesce(parcel.land_type::text,'?'),','
                      ORDER BY parcel.land_type NULLS FIRST,parcel.id)
    FROM land_parcels parcel
    WHERE parcel.transaction_id=subject_id
  ),''),']')
$$;

CREATE OR REPLACE FUNCTION analytics.rental_package_signature(subject_id bigint)
RETURNS text
LANGUAGE sql
STABLE
AS $$
SELECT concat('spaces[',coalesce((
  SELECT string_agg(coalesce(space.rented_spaces_type::text,'?'),','
                    ORDER BY space.rented_spaces_type NULLS FIRST,space.id)
  FROM rental_building_parts space
  WHERE space.transaction_id=subject_id
),''),']')
$$;

COMMENT ON FUNCTION analytics.sale_package_signature(bigint) IS
  'Deterministic ETN building-part and parcel type multiset used for comparable-package matching.';
COMMENT ON FUNCTION analytics.rental_package_signature(bigint) IS
  'Deterministic ETN rental-space type multiset used for comparable-package matching.';

GRANT EXECUTE ON FUNCTION analytics.sale_package_signature(bigint) TO real_estate_api;
GRANT EXECUTE ON FUNCTION analytics.rental_package_signature(bigint) TO real_estate_api;
