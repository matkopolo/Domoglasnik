package analytics

import (
	"context"
	"errors"
	"fmt"
)

var ErrAnomalyNotFound = errors.New("anomaly signal not found")

type AnomalyFilter struct {
	TransactionKind string
	Flag            string
	ReviewStatus    string
	MinimumScore    float64
}

type AnomalySignal struct {
	TransactionKind      string   `json:"transactionKind"`
	TransactionID        int64    `json:"transactionId"`
	Flags                []string `json:"flags"`
	HardRuleScore        float64  `json:"hardRuleScore"`
	RobustCohortScore    float64  `json:"robustCohortScore"`
	ModelResidualScore   *float64 `json:"modelResidualScore"`
	IsolationForestScore *float64 `json:"isolationForestScore"`
	DuplicateScore       float64  `json:"duplicateScore"`
	SpatialScore         float64  `json:"spatialScore"`
	ReviewScore          float64  `json:"reviewScore"`
	SignalVersion        string   `json:"signalVersion"`
	ModelVersion         *string  `json:"modelVersion"`
	ReviewStatus         string   `json:"reviewStatus"`
	ReviewNotes          *string  `json:"reviewNotes"`
	ReviewedBy           *string  `json:"reviewedBy"`
	ReviewedAt           *string  `json:"reviewedAt"`
	EvaluatedAt          string   `json:"evaluatedAt"`
}

type AnomalySummary struct {
	Total        int64 `json:"total"`
	Flagged      int64 `json:"flagged"`
	Unreviewed   int64 `json:"unreviewed"`
	HighPriority int64 `json:"highPriority"`
}

type AnomalyStore interface {
	Anomalies(context.Context, AnomalyFilter, int, int) ([]AnomalySignal, int64, error)
	AnomalySummary(context.Context, string) (AnomalySummary, error)
	ReviewAnomaly(context.Context, string, int64, string, *string, string) error
}

func (r *StatisticsRepository) ReviewAnomaly(
	ctx context.Context, kind string, id int64, status string, notes *string,
	reviewerID string,
) error {
	command, err := r.pool.Exec(ctx, `
		UPDATE analytics.anomaly_signals
		SET review_status=$3,review_notes=$4,reviewed_by=$5,reviewed_at=now()
		WHERE transaction_kind=$1 AND transaction_id=$2`,
		kind, id, status, notes, reviewerID)
	if err != nil {
		return fmt.Errorf("review anomaly: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrAnomalyNotFound
	}
	return nil
}

func (r *StatisticsRepository) Anomalies(
	ctx context.Context, filter AnomalyFilter, limit, offset int,
) ([]AnomalySignal, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT count(*) OVER()::bigint,transaction_kind,transaction_id,flags,
		  hard_rule_score::float8,robust_cohort_score::float8,
		  model_residual_score::float8,isolation_forest_score::float8,
		  duplicate_score::float8,spatial_score::float8,review_score::float8,
		  signal_version,model_version,review_status,review_notes,reviewed_by,
		  reviewed_at::text,evaluated_at::text
		FROM analytics.anomaly_signals
		WHERE ($1='' OR transaction_kind=$1)
		  AND ($2='' OR flags @> ARRAY[$2]::text[])
		  AND ($3='' OR review_status=$3)
		  AND review_score >= $4
		ORDER BY review_score DESC,transaction_kind,transaction_id
		LIMIT $5 OFFSET $6`, filter.TransactionKind, filter.Flag,
		filter.ReviewStatus, filter.MinimumScore, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list anomaly signals: %w", err)
	}
	defer rows.Close()
	items := make([]AnomalySignal, 0)
	var total int64
	for rows.Next() {
		var item AnomalySignal
		if err := rows.Scan(&total, &item.TransactionKind, &item.TransactionID,
			&item.Flags, &item.HardRuleScore, &item.RobustCohortScore,
			&item.ModelResidualScore, &item.IsolationForestScore,
			&item.DuplicateScore, &item.SpatialScore, &item.ReviewScore,
			&item.SignalVersion, &item.ModelVersion, &item.ReviewStatus,
			&item.ReviewNotes, &item.ReviewedBy, &item.ReviewedAt,
			&item.EvaluatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan anomaly signal: %w", err)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *StatisticsRepository) AnomalySummary(
	ctx context.Context, transactionKind string,
) (AnomalySummary, error) {
	var result AnomalySummary
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)::bigint,
		  count(*) FILTER(WHERE cardinality(flags)>0)::bigint,
		  count(*) FILTER(WHERE review_status='unreviewed'
		    AND cardinality(flags)>0)::bigint,
		  count(*) FILTER(WHERE review_score>=0.30)::bigint
		FROM analytics.anomaly_signals
		WHERE ($1='' OR transaction_kind=$1)`, transactionKind).Scan(
		&result.Total, &result.Flagged, &result.Unreviewed, &result.HighPriority)
	if err != nil {
		return AnomalySummary{}, fmt.Errorf("summarize anomaly signals: %w", err)
	}
	return result, nil
}
