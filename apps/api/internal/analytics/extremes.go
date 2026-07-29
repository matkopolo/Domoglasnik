package analytics

import (
	"context"
	"fmt"
)

type TransactionExtreme struct {
	Direction     string   `json:"direction"`
	TransactionID int64    `json:"transactionId"`
	ContractDate  *string  `json:"contractDate"`
	PropertyGroup string   `json:"propertyGroup"`
	Area          *float64 `json:"area"`
	Amount        *float64 `json:"amount"`
	AmountPerSQM  float64  `json:"amountPerSqm"`
}

func (r *StatisticsRepository) TransactionExtremes(
	ctx context.Context, kind string, filter StatisticsFilter, limit int,
) ([]TransactionExtreme, error) {
	table, amount, perSQM := saleFactsTable(filter), "price", "price_per_sqm"
	if kind == "rent" {
		table, amount, perSQM = "analytics.rental_facts",
			"monthly_rent_ex_vat", "monthly_rent_per_sqm"
	} else if kind != "sale" {
		return nil, fmt.Errorf("unsupported transaction kind %q", kind)
	}
	clause, geographyID, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		WITH eligible AS MATERIALIZED (
		  SELECT transaction_id,contract_date,property_group,area,%s amount,
		    %s amount_per_sqm
		  FROM %s
		  WHERE representative_eligible AND %s>0 AND %s>0
		    AND ($1::date IS NULL OR contract_date >= $1)
		    AND ($2::date IS NULL OR contract_date <= $2)
		    AND ($3='' OR property_group=$3) AND %s
		), ranked AS (
		  SELECT *,
		    row_number() OVER(ORDER BY amount,transaction_id) low_rank,
		    row_number() OVER(ORDER BY amount DESC,transaction_id) high_rank
		  FROM eligible
		)
		SELECT *
		FROM (
		  SELECT 'lowest' direction,transaction_id,contract_date::text,property_group,
		    area::float8,amount::float8,amount_per_sqm::float8
		  FROM ranked WHERE low_rank<=$7
		  UNION ALL
		  SELECT 'highest' direction,transaction_id,contract_date::text,property_group,
		    area::float8,amount::float8,amount_per_sqm::float8
		  FROM ranked WHERE high_rank<=$7
		) extremes
		ORDER BY CASE direction WHEN 'highest' THEN 0 ELSE 1 END,
		  CASE WHEN direction='highest' THEN -amount ELSE amount END`,
		amount, perSQM, table, perSQM, amount, clause)
	rows, err := r.pool.Query(ctx, query, filter.FromDate, filter.ToDate,
		filter.PropertyGroup, geographyID, filter.MinArea, filter.MaxArea, limit)
	if err != nil {
		return nil, fmt.Errorf("load transaction extremes: %w", err)
	}
	defer rows.Close()
	items := make([]TransactionExtreme, 0, limit*2)
	for rows.Next() {
		var item TransactionExtreme
		if err := rows.Scan(&item.Direction, &item.TransactionID,
			&item.ContractDate, &item.PropertyGroup, &item.Area,
			&item.Amount, &item.AmountPerSQM); err != nil {
			return nil, fmt.Errorf("scan transaction extreme: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
