CREATE OR REPLACE FUNCTION analytics.refresh_near_duplicate_candidates()
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    affected bigint;
BEGIN
    TRUNCATE analytics.near_duplicate_candidates;
    INSERT INTO analytics.near_duplicate_candidates
    WITH facts AS (
      SELECT 'sale'::text kind,transaction_id,contract_date,area,
        price amount,cadastral_municipality_code,building_number,
        building_part_number,property_group
      FROM analytics.sale_facts
      UNION ALL
      SELECT 'rent',transaction_id,contract_date,area,
        monthly_rent_ex_vat,cadastral_municipality_code,building_number,
        building_part_number,property_group
      FROM analytics.rental_facts
    )
    SELECT left_fact.kind,left_fact.transaction_id,right_fact.transaction_id,
      'same_building_part',
      abs(left_fact.contract_date-right_fact.contract_date),
      abs(left_fact.area-right_fact.area)/greatest(left_fact.area,right_fact.area),
      abs(left_fact.amount-right_fact.amount)/greatest(left_fact.amount,right_fact.amount),
      greatest(0,1-
        abs(left_fact.contract_date-right_fact.contract_date)/7.0-
        abs(left_fact.area-right_fact.area)/greatest(left_fact.area,right_fact.area)-
        abs(left_fact.amount-right_fact.amount)/greatest(left_fact.amount,right_fact.amount)
      ),
      'near-duplicate-v2',now()
    FROM facts left_fact
    JOIN facts right_fact
      ON right_fact.kind=left_fact.kind
     AND right_fact.transaction_id>left_fact.transaction_id
     AND right_fact.property_group=left_fact.property_group
     AND right_fact.cadastral_municipality_code=left_fact.cadastral_municipality_code
     AND right_fact.building_number=left_fact.building_number
     AND right_fact.building_part_number=left_fact.building_part_number
     AND right_fact.contract_date BETWEEN
         left_fact.contract_date-7 AND left_fact.contract_date+7
    WHERE left_fact.contract_date IS NOT NULL
      AND left_fact.area>0 AND right_fact.area>0
      AND left_fact.amount>0 AND right_fact.amount>0
      AND abs(left_fact.area-right_fact.area)/
          greatest(left_fact.area,right_fact.area)<=.02
      AND abs(left_fact.amount-right_fact.amount)/
          greatest(left_fact.amount,right_fact.amount)<=.02;
    GET DIAGNOSTICS affected=ROW_COUNT;

    UPDATE analytics.anomaly_signals
    SET duplicate_score=0,
        flags=array_remove(array_remove(flags,'possible_near_duplicate'),'possible_duplicate'),
        review_score=least(1.0,
          .35*hard_rule_score+.20*robust_cohort_score+
          .20*coalesce(model_residual_score,0)+
          .10*coalesce(isolation_forest_score,0)+.05*spatial_score);

    WITH exact_duplicates AS (
      SELECT 'sale'::text transaction_kind,transaction_id
      FROM (
        SELECT transaction_id,row_number() OVER (
          PARTITION BY contract_date,property_group,area,price,
                       cadastral_municipality_code,building_number,building_part_number
          ORDER BY transaction_id
        ) duplicate_position
        FROM analytics.sale_facts
        WHERE contract_date IS NOT NULL AND area IS NOT NULL AND price IS NOT NULL
      ) ranked WHERE duplicate_position>1
      UNION ALL
      SELECT 'rent',transaction_id
      FROM (
        SELECT transaction_id,row_number() OVER (
          PARTITION BY contract_date,property_group,area,monthly_rent_ex_vat,
                       cadastral_municipality_code,building_number,building_part_number
          ORDER BY transaction_id
        ) duplicate_position
        FROM analytics.rental_facts
        WHERE contract_date IS NOT NULL AND area IS NOT NULL
          AND monthly_rent_ex_vat IS NOT NULL
      ) ranked WHERE duplicate_position>1
    )
    UPDATE analytics.anomaly_signals signal
    SET duplicate_score=1,
        flags=array_append(signal.flags,'possible_duplicate'),
        review_score=least(1.0,
          .35*signal.hard_rule_score+.20*signal.robust_cohort_score+
          .20*coalesce(signal.model_residual_score,0)+
          .10*coalesce(signal.isolation_forest_score,0)+.10+
          .05*signal.spatial_score)
    FROM exact_duplicates duplicate
    WHERE duplicate.transaction_kind=signal.transaction_kind
      AND duplicate.transaction_id=signal.transaction_id;

    UPDATE analytics.anomaly_signals signal
    SET duplicate_score=greatest(signal.duplicate_score,candidate.candidate_score),
        flags=CASE WHEN 'possible_near_duplicate'=ANY(signal.flags)
          THEN signal.flags ELSE array_append(signal.flags,'possible_near_duplicate') END,
        review_score=least(1.0,
          .35*signal.hard_rule_score+.20*signal.robust_cohort_score+
          .20*coalesce(signal.model_residual_score,0)+
          .10*coalesce(signal.isolation_forest_score,0)+
          .10*greatest(signal.duplicate_score,candidate.candidate_score)+
          .05*signal.spatial_score)
    FROM (
      SELECT transaction_kind,candidate_transaction_id transaction_id,
             max(candidate_score) candidate_score
      FROM analytics.near_duplicate_candidates
      GROUP BY 1,2
    ) candidate
    WHERE candidate.transaction_kind=signal.transaction_kind
      AND candidate.transaction_id=signal.transaction_id;
    ANALYZE analytics.near_duplicate_candidates;
    RETURN affected;
END
$$;

COMMENT ON FUNCTION analytics.refresh_near_duplicate_candidates() IS
  'Refreshes possible near-duplicate pairs using a conservative seven-day window; flags are probabilistic, not confirmed errors.';
