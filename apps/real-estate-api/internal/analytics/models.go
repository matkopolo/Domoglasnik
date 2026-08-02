package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrPredictionNotFound = errors.New("valuation prediction not found")

type MarketIndexFilter struct {
	TransactionKind string
	Method          string
	PropertyGroup   string
}

type MarketIndexPoint struct {
	TransactionKind     string   `json:"transactionKind"`
	Method              string   `json:"method"`
	PropertyGroup       string   `json:"propertyGroup"`
	Period              string   `json:"period"`
	IndexValue          float64  `json:"indexValue"`
	SampleSize          int      `json:"sampleSize"`
	BasePeriod          string   `json:"basePeriod"`
	MethodologyVersion  string   `json:"methodologyVersion"`
	ModelVersion        *string  `json:"modelVersion"`
	CalculationRunID    string   `json:"calculationRunId"`
	ObservedSampleSize  int      `json:"observedSampleSize"`
	ReferenceSampleSize *int     `json:"referenceSampleSize"`
	LowerIndexValue     *float64 `json:"lowerIndexValue"`
	UpperIndexValue     *float64 `json:"upperIndexValue"`
	NominalCoverage     *float64 `json:"nominalCoverage"`
	UncertaintyMethod   *string  `json:"uncertaintyMethod"`
	IsPublishable       bool     `json:"isPublishable"`
	SuppressionReason   *string  `json:"suppressionReason"`
}

type ValuationPrediction struct {
	TransactionKind       string          `json:"transactionKind"`
	TransactionID         int64           `json:"transactionId"`
	PropertyGroup         string          `json:"propertyGroup"`
	Area                  float64         `json:"area"`
	ActualAmountPerSQM    float64         `json:"actualAmountPerSqm"`
	PredictedAmountPerSQM float64         `json:"predictedAmountPerSqm"`
	LowerAmountPerSQM     float64         `json:"lowerAmountPerSqm"`
	UpperAmountPerSQM     float64         `json:"upperAmountPerSqm"`
	PredictedTotal        float64         `json:"predictedTotal"`
	LowerTotal            float64         `json:"lowerTotal"`
	UpperTotal            float64         `json:"upperTotal"`
	LogResidual           float64         `json:"logResidual"`
	ModelVersion          string          `json:"modelVersion"`
	ModelType             string          `json:"modelType"`
	TrainingRows          int             `json:"trainingRows"`
	Metrics               json.RawMessage `json:"metrics"`
	Reliability           string          `json:"reliability"`
	Warnings              []string        `json:"warnings"`
}

type ModelStore interface {
	MarketIndices(context.Context, MarketIndexFilter) ([]MarketIndexPoint, error)
	ValuationPrediction(context.Context, string, int64) (ValuationPrediction, error)
}

func (r *StatisticsRepository) MarketIndices(
	ctx context.Context, filter MarketIndexFilter,
) ([]MarketIndexPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT index_data.transaction_kind,index_data.method,index_data.property_group,
		  index_data.period::text,index_data.index_value::float8,
		  index_data.sample_size,index_data.base_period::text,
		  index_data.methodology_version,model.model_version,
		  index_data.calculation_run_id::text,index_data.observed_sample_size,
		  index_data.reference_sample_size,index_data.lower_index_value::float8,
		  index_data.upper_index_value::float8,index_data.nominal_coverage::float8,
		  index_data.uncertainty_method,index_data.is_publishable,
		  index_data.suppression_reason
		FROM analytics.current_market_indices index_data
		LEFT JOIN analytics.model_runs model ON model.id=index_data.model_run_id
		WHERE index_data.geography_id IS NULL
		  AND ($1='' OR index_data.transaction_kind=$1)
		  AND ($2='' OR index_data.method=$2)
		  AND ($3='' OR index_data.property_group=$3)
		ORDER BY index_data.transaction_kind,index_data.method,
		         index_data.property_group,index_data.period`,
		filter.TransactionKind, filter.Method, filter.PropertyGroup)
	if err != nil {
		return nil, fmt.Errorf("list market indices: %w", err)
	}
	defer rows.Close()
	items := make([]MarketIndexPoint, 0)
	for rows.Next() {
		var item MarketIndexPoint
		if err := rows.Scan(&item.TransactionKind, &item.Method,
			&item.PropertyGroup, &item.Period, &item.IndexValue, &item.SampleSize,
			&item.BasePeriod, &item.MethodologyVersion, &item.ModelVersion,
			&item.CalculationRunID, &item.ObservedSampleSize,
			&item.ReferenceSampleSize, &item.LowerIndexValue,
			&item.UpperIndexValue, &item.NominalCoverage,
			&item.UncertaintyMethod, &item.IsPublishable,
			&item.SuppressionReason); err != nil {
			return nil, fmt.Errorf("scan market index: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *StatisticsRepository) ValuationPrediction(
	ctx context.Context, kind string, id int64,
) (ValuationPrediction, error) {
	factTable := map[string]string{
		"sale": "analytics.sale_facts",
		"rent": "analytics.rental_facts",
	}[kind]
	if factTable == "" {
		return ValuationPrediction{}, ErrPredictionNotFound
	}
	query := fmt.Sprintf(`
		SELECT prediction.transaction_kind,prediction.transaction_id,
		  fact.property_group,fact.area::float8,
		  prediction.actual_amount_per_sqm::float8,
		  prediction.predicted_amount_per_sqm::float8,
		  prediction.lower_amount_per_sqm::float8,
		  prediction.upper_amount_per_sqm::float8,
		  prediction.log_residual::float8,model.model_version,model.model_type,
		  model.training_rows,model.metrics,
		  COALESCE(
		    (model.metrics->'property_groups'->fact.property_group->'temporal'
		      ->>'median_absolute_percentage_error')::float8,
		    (model.metrics->'property_groups'->fact.property_group->'spatial'
		      ->>'median_absolute_percentage_error')::float8,
		    (model.metrics->'property_groups'->fact.property_group
		      ->>'median_absolute_percentage_error')::float8,
		    (model.metrics->>'median_absolute_percentage_error')::float8)
		FROM analytics.valuation_predictions prediction
		JOIN %s fact ON fact.transaction_id=prediction.transaction_id
		JOIN analytics.model_runs model ON model.id=prediction.model_run_id
		WHERE prediction.transaction_kind=$1 AND prediction.transaction_id=$2`,
		factTable)
	var result ValuationPrediction
	var medianAPE *float64
	err := r.pool.QueryRow(ctx, query, kind, id).Scan(
		&result.TransactionKind, &result.TransactionID, &result.PropertyGroup,
		&result.Area, &result.ActualAmountPerSQM,
		&result.PredictedAmountPerSQM, &result.LowerAmountPerSQM,
		&result.UpperAmountPerSQM, &result.LogResidual, &result.ModelVersion,
		&result.ModelType, &result.TrainingRows, &result.Metrics, &medianAPE)
	if errors.Is(err, pgx.ErrNoRows) {
		return ValuationPrediction{}, ErrPredictionNotFound
	}
	if err != nil {
		return ValuationPrediction{}, fmt.Errorf("load valuation prediction: %w", err)
	}
	result.PredictedTotal = result.PredictedAmountPerSQM * result.Area
	result.LowerTotal = result.LowerAmountPerSQM * result.Area
	result.UpperTotal = result.UpperAmountPerSQM * result.Area
	result.Warnings = []string{
		"statistical estimate, not an official valuation",
		"range is empirical and does not include every property-specific condition",
	}
	switch {
	case medianAPE == nil:
		result.Reliability = "very_low"
		result.Warnings = append(result.Warnings,
			"model error is unavailable for this property group")
	case *medianAPE <= 0.25:
		result.Reliability = "moderate"
	case *medianAPE <= 0.45:
		result.Reliability = "low"
	default:
		result.Reliability = "very_low"
		result.Warnings = append(result.Warnings,
			"model error for this property group is too high for decision-grade valuation")
	}
	return result, nil
}
