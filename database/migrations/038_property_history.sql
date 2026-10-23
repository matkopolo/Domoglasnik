-- Read-only evidence, independent of the published repeat-sales index methodology.
CREATE INDEX IF NOT EXISTS land_parcels_history_identity_idx
    ON land_parcels(cadastral_municipality_code,parcel_number,transaction_id);
CREATE INDEX IF NOT EXISTS building_parts_history_identity_idx
    ON building_parts(cadastral_municipality_code,building_number,building_part_number,transaction_id);
CREATE INDEX IF NOT EXISTS building_parts_history_parcel_idx
    ON building_parts(cadastral_municipality_code,parcel_number_for_geolocation,transaction_id);

CREATE OR REPLACE VIEW analytics.property_history_events AS
WITH components AS (
    SELECT 'building_part'::text AS identity_kind,bp.cadastral_municipality_code,
           bp.building_number AS identity_number,bp.building_part_number AS part_number,
           bp.transaction_id,bp.id AS component_id,'exact_identity'::text AS evidence,
           bp.sold_share,bp.building_part_type AS source_type,
           coalesce(bp.sold_area,bp.sold_building_part_area,bp.sold_usable_area)::float8 AS component_area,
           bp.construction_year,bp.contractual_price::float8 AS component_price
    FROM building_parts bp
    WHERE bp.cadastral_municipality_code>0 AND bp.building_number ~ '^[1-9][0-9]*$'
      AND bp.building_part_number ~ '^[1-9][0-9]*$'
    UNION ALL
    SELECT 'parcel',lp.cadastral_municipality_code,lp.parcel_number,'',
           lp.transaction_id,lp.id,'exact_identity',lp.sold_share,lp.land_type,
           lp.parcel_area::float8,NULL,lp.contractual_price::float8
    FROM land_parcels lp
    WHERE lp.cadastral_municipality_code>0 AND lp.parcel_number ~ '^[1-9][0-9]*(/[1-9][0-9]*)?$'
    UNION ALL
    SELECT 'parcel',bp.cadastral_municipality_code,bp.parcel_number_for_geolocation,'',
           bp.transaction_id,bp.id,'building_geolocation_reference',bp.sold_share,
           bp.building_part_type,coalesce(bp.sold_area,bp.sold_building_part_area,bp.sold_usable_area)::float8,
           bp.construction_year,bp.contractual_price::float8
    FROM building_parts bp
    WHERE bp.cadastral_municipality_code>0
      AND bp.parcel_number_for_geolocation ~ '^[1-9][0-9]*(/[1-9][0-9]*)?$'
)
SELECT c.*,f.contract_date,f.property_group,f.price::float8 AS agreement_price,
       f.building_part_count+f.land_parcel_count AS component_count,
       f.is_quality_eligible,f.representative_eligible,
       coalesce(s.flags && ARRAY['possible_duplicate','possible_near_duplicate'],false) AS possible_duplicate,
       f.statistical_region_id,f.municipality_id,f.city_district_id,f.local_community_id,f.settlement_id
FROM components c JOIN analytics.sale_facts f USING(transaction_id)
LEFT JOIN analytics.anomaly_signals s ON s.transaction_kind='sale' AND s.transaction_id=c.transaction_id;

-- Collapse repeated source component rows within a contract before sequencing. Ambiguous
-- events remain visible, but cannot contribute a price/holding-period statistic.
CREATE OR REPLACE VIEW analytics.property_resale_pairs AS
WITH events AS (
    SELECT identity_kind,cadastral_municipality_code,identity_number,part_number,
           transaction_id,min(contract_date) AS contract_date,min(property_group) AS property_group,
           min(agreement_price) AS price,min(component_area) AS area,min(source_type) AS source_type,
           bool_and(is_quality_eligible AND representative_eligible AND NOT possible_duplicate
             AND component_count=1 AND replace(coalesce(sold_share,''),' ','') IN ('1','1/1','1.0','100/100')
             AND agreement_price>0 AND component_area>0) AND count(*)=1 AS eligible,
           min(statistical_region_id) AS statistical_region_id,min(municipality_id) AS municipality_id,
           min(city_district_id) AS city_district_id,min(local_community_id) AS local_community_id,
           min(settlement_id) AS settlement_id
    FROM analytics.property_history_events WHERE evidence='exact_identity'
    GROUP BY identity_kind,cadastral_municipality_code,identity_number,part_number,transaction_id
), sequenced AS (
    SELECT events.*,lag(transaction_id) OVER w AS previous_transaction_id,
           lag(contract_date) OVER w AS previous_date,lag(price) OVER w AS previous_price,
           lag(area) OVER w AS previous_area,lag(source_type) OVER w AS previous_type,
           lag(eligible) OVER w AS previous_eligible
    FROM events WINDOW w AS (PARTITION BY identity_kind,cadastral_municipality_code,identity_number,part_number
                            ORDER BY contract_date NULLS LAST,transaction_id)
)
SELECT *,contract_date-previous_date AS interval_days,
       coalesce(eligible AND previous_eligible AND contract_date>previous_date
           AND source_type=previous_type AND abs(area/NULLIF(previous_area,0)-1)<=0.01,false) AS comparable,
       CASE WHEN identity_kind='parcel' AND previous_type IN (7,8) AND source_type IN (1,2)
                 AND contract_date>previous_date THEN true ELSE false END AS agricultural_to_buildable
FROM sequenced WHERE previous_transaction_id IS NOT NULL;

GRANT SELECT ON analytics.property_history_events,analytics.property_resale_pairs TO real_estate_api;
COMMENT ON VIEW analytics.property_resale_pairs IS
 'Observed consecutive ETN sales, not ownership history or net profit. No 180-day cutoff; conservative complete-share single-component same-type/area pairs only for published summaries.';
