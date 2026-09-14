package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Source identities are not cadastral lineage or proof of uninterrupted ownership.
type PropertyHistory struct {
	TransactionID int64          `json:"transactionId"`
	Assets        []HistoryAsset `json:"assets"`
	Truncated     bool           `json:"truncated"`
}

type HistoryAsset struct {
	Kind          string         `json:"kind"`
	CadastralCode int32          `json:"cadastralCode"`
	Number        string         `json:"number"`
	PartNumber    string         `json:"partNumber"`
	Events        []HistoryEvent `json:"events"`
	Pairs         []ResalePair   `json:"pairs"`
	Truncated     bool           `json:"truncated"`
}

type HistoryEvent struct {
	TransactionID     int64    `json:"transactionId"`
	ComponentID       int64    `json:"componentId"`
	ContractDate      *string  `json:"contractDate"`
	Evidence          string   `json:"evidence"`
	PropertyGroup     string   `json:"propertyGroup"`
	SourceType        *int32   `json:"sourceType"`
	Share             *string  `json:"share"`
	Area              *float64 `json:"area"`
	AgreementPrice    *float64 `json:"agreementPrice"`
	ComponentPrice    *float64 `json:"componentPrice"`
	ConstructionYear  *int32   `json:"constructionYear"`
	ComponentCount    int      `json:"componentCount"`
	Representative    bool     `json:"representative"`
	PossibleDuplicate bool     `json:"possibleDuplicate"`
}

type ResalePair struct {
	TransactionID  int64    `json:"transactionId"`
	PreviousID     int64    `json:"previousId"`
	From           *string  `json:"from"`
	To             *string  `json:"to"`
	Days           *int     `json:"days"`
	Comparable     bool     `json:"comparable"`
	PreviousPrice  *float64 `json:"previousPrice"`
	Price          *float64 `json:"price"`
	NominalChange  *float64 `json:"nominalChange"`
	ChangePercent  *float64 `json:"changePercent"`
	LandTransition bool     `json:"landTransition"`
}

type ResaleSummary struct {
	PairCount     int64    `json:"pairCount"`
	AssetCount    int64    `json:"assetCount"`
	P25Days       *float64 `json:"p25Days"`
	MedianDays    *float64 `json:"medianDays"`
	P75Days       *float64 `json:"p75Days"`
	IsPublishable bool     `json:"isPublishable"`
}

type HistoryStore interface {
	PropertyHistory(context.Context, int64) (PropertyHistory, error)
	ResaleSummary(context.Context, StatisticsFilter) (ResaleSummary, error)
}

func (r *StatisticsRepository) PropertyHistory(ctx context.Context, id int64) (PropertyHistory, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PropertyHistory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := loadPropertyHistory(ctx, tx, id)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func loadPropertyHistory(ctx context.Context, tx pgx.Tx, id int64) (PropertyHistory, error) {
	result := PropertyHistory{TransactionID: id, Assets: []HistoryAsset{}}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM analytics.sale_facts WHERE transaction_id=$1)", id).Scan(&exists); err != nil {
		return result, err
	}
	if !exists {
		return result, ErrTransactionNotFound
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT identity_kind,cadastral_municipality_code,identity_number,part_number
		FROM analytics.property_history_events WHERE transaction_id=$1
		ORDER BY identity_kind,cadastral_municipality_code,identity_number,part_number LIMIT 51`, id)
	if err != nil {
		return result, fmt.Errorf("history identities: %w", err)
	}
	for rows.Next() {
		asset := HistoryAsset{Events: []HistoryEvent{}, Pairs: []ResalePair{}}
		if err := rows.Scan(&asset.Kind, &asset.CadastralCode, &asset.Number, &asset.PartNumber); err != nil {
			rows.Close()
			return result, err
		}
		result.Assets = append(result.Assets, asset)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Assets) > 50 {
		result.Truncated = true
		result.Assets = result.Assets[:50]
	}
	for i := range result.Assets {
		a := &result.Assets[i]
		args := []any{a.Kind, a.CadastralCode, a.Number, a.PartNumber}
		// Bound unusually large parcel histories. Never claim a truncated history is complete.
		events, err := tx.Query(ctx, `SELECT transaction_id,component_id,contract_date::text,evidence,property_group,
			source_type,sold_share,component_area,agreement_price,component_price,construction_year,component_count,
			representative_eligible,possible_duplicate
			FROM analytics.property_history_events
			WHERE identity_kind=$1 AND cadastral_municipality_code=$2 AND identity_number=$3 AND part_number=$4
			ORDER BY contract_date NULLS LAST,transaction_id,evidence,component_id LIMIT 201`, args...)
		if err != nil {
			return result, err
		}
		for events.Next() {
			var e HistoryEvent
			if err := events.Scan(&e.TransactionID, &e.ComponentID, &e.ContractDate, &e.Evidence, &e.PropertyGroup,
				&e.SourceType, &e.Share, &e.Area, &e.AgreementPrice, &e.ComponentPrice, &e.ConstructionYear, &e.ComponentCount,
				&e.Representative, &e.PossibleDuplicate); err != nil {
				events.Close()
				return result, err
			}
			a.Events = append(a.Events, e)
		}
		events.Close()
		if err := events.Err(); err != nil {
			return result, err
		}
		if len(a.Events) > 200 {
			a.Truncated = true
			a.Events = a.Events[:200]
			continue
		}
		var data []byte
		err = tx.QueryRow(ctx, `SELECT coalesce(jsonb_agg(jsonb_build_object(
			'transactionId',transaction_id,'previousId',previous_transaction_id,'from',previous_date,'to',contract_date,
			'days',interval_days,'comparable',comparable,'previousPrice',previous_price,'price',price,
			'nominalChange',CASE WHEN comparable THEN price-previous_price END,
			'changePercent',CASE WHEN comparable THEN 100*(price/NULLIF(previous_price,0)-1) END,
			'landTransition',agricultural_to_buildable
		) ORDER BY contract_date NULLS LAST,transaction_id),'[]'::jsonb)
		FROM analytics.property_resale_pairs
		WHERE identity_kind=$1 AND cadastral_municipality_code=$2 AND identity_number=$3 AND part_number=$4`, args...).Scan(&data)
		if err != nil {
			return result, err
		}
		if err := json.Unmarshal(data, &a.Pairs); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r *StatisticsRepository) ResaleSummary(ctx context.Context, filter StatisticsFilter) (ResaleSummary, error) {
	clause, geo, err := r.geographyClause(ctx, filter.GeographyID)
	if err != nil {
		return ResaleSummary{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return ResaleSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Country-wide history can otherwise exhaust Docker's /dev/shm through
	// parallel hash aggregation. Keep this optional report bounded to one worker;
	// this changes neither the population nor other requests' planner settings.
	if _, err := tx.Exec(ctx, "SET LOCAL max_parallel_workers_per_gather=0"); err != nil {
		return ResaleSummary{}, err
	}
	result, err := loadResaleSummary(ctx, tx, filter, clause, geo)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func loadResaleSummary(ctx context.Context, tx pgx.Tx, filter StatisticsFilter, clause string, geo *int64) (ResaleSummary, error) {
	// Restrict the later sale only: its predecessor may predate the selected period.
	query := fmt.Sprintf(`SELECT count(*),count(DISTINCT (identity_kind,cadastral_municipality_code,identity_number,part_number)),
		CASE WHEN count(*)>=30 THEN percentile_cont(0.25) WITHIN GROUP(ORDER BY interval_days) END,
		CASE WHEN count(*)>=30 THEN percentile_cont(0.5) WITHIN GROUP(ORDER BY interval_days) END,
		CASE WHEN count(*)>=30 THEN percentile_cont(0.75) WITHIN GROUP(ORDER BY interval_days) END,count(*)>=30
		FROM analytics.property_resale_pairs WHERE comparable
		AND ($1::date IS NULL OR contract_date >= $1) AND ($2::date IS NULL OR contract_date <= $2)
		AND ($3='' OR property_group=$3) AND %s
		AND ($5::float8 IS NULL OR area >= $5) AND ($6::float8 IS NULL OR area <= $6)
		AND transaction_id IN (SELECT transaction_id FROM %s)`, clause, saleFactsTable(filter))
	var result ResaleSummary
	err := tx.QueryRow(ctx, query, filter.FromDate, filter.ToDate, filter.PropertyGroup, geo, filter.MinArea, filter.MaxArea).
		Scan(&result.PairCount, &result.AssetCount, &result.P25Days, &result.MedianDays, &result.P75Days, &result.IsPublishable)
	return result, err
}
