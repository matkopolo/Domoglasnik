package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ContextBenchmark struct {
	Scope              string  `json:"scope"`
	Label              string  `json:"label"`
	GeographyID        *int64  `json:"geographyId,omitempty"`
	MedianAmountPerSQM float64 `json:"medianAmountPerSqm"`
	SampleSize         int64   `json:"sampleSize"`
	DifferencePercent  float64 `json:"differencePercent"`
	Percentile         float64 `json:"percentile"`
	Period             int     `json:"period"`
	RadiusMeters       *int    `json:"radiusMeters,omitempty"`
	Reliability        string  `json:"reliability"`
	IsPublishable      bool    `json:"isPublishable"`
}

type TransactionContext struct {
	TransactionKind    string             `json:"transactionKind"`
	TransactionID      int64              `json:"transactionId"`
	ContractYear       int                `json:"contractYear"`
	IncompletePeriod   bool               `json:"incompletePeriod"`
	PropertyGroup      string             `json:"propertyGroup"`
	ActualAmountPerSQM float64            `json:"actualAmountPerSqm"`
	Benchmarks         []ContextBenchmark `json:"benchmarks"`
	Valuation          *ContextValuation  `json:"valuation"`
	Quality            QualityDecision    `json:"quality"`
	OutlierStatus      *string            `json:"outlierStatus"`
	MethodologyVersion string             `json:"methodologyVersion"`
}

// ContextValuation is the public, decision-useful subset of the model result.
// Training diagnostics remain available from the dedicated valuation endpoint.
type ContextValuation struct {
	PredictedAmountPerSQM float64  `json:"predictedAmountPerSqm"`
	LowerAmountPerSQM     float64  `json:"lowerAmountPerSqm"`
	UpperAmountPerSQM     float64  `json:"upperAmountPerSqm"`
	PredictedTotal        float64  `json:"predictedTotal"`
	LowerTotal            float64  `json:"lowerTotal"`
	UpperTotal            float64  `json:"upperTotal"`
	ModelVersion          string   `json:"modelVersion"`
	Reliability           string   `json:"reliability"`
	Warnings              []string `json:"warnings"`
}

func benchmarkDifference(actual, median float64) (float64, bool) {
	if actual <= 0 || median <= 0 {
		return 0, false
	}
	return (actual/median - 1) * 100, true
}

func benchmarkReliability(sample int64) string {
	switch {
	case sample < 5:
		return "unavailable"
	case sample < 10:
		return "very_low"
	case sample < 30:
		return "low"
	default:
		return "moderate"
	}
}

type contextSubject struct {
	year             int
	propertyGroup    string
	actual           float64
	quality          QualityDecision
	outlierStatus    *string
	regionID         *int64
	municipalityID   *int64
	cityDistrictID   *int64
	localCommunityID *int64
	settlementID     *int64
}

func (r *StatisticsRepository) TransactionContext(
	ctx context.Context, kind string, id int64,
) (TransactionContext, error) {
	factTable, amountColumn := "analytics.sale_facts", "price_per_sqm"
	cohortTable := "analytics.sale_facts_strict_land"
	outlierTable := "analytics.sale_outlier_evaluations"
	if kind == "rent" {
		factTable, cohortTable = "analytics.rental_facts", "analytics.rental_facts"
		amountColumn = "monthly_rent_per_sqm"
		outlierTable = "analytics.rental_outlier_evaluations"
	} else if kind != "sale" {
		return TransactionContext{}, ErrTransactionNotFound
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return TransactionContext{}, fmt.Errorf("begin transaction context: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var subject contextSubject
	query := fmt.Sprintf(`
		SELECT extract(year FROM fact.contract_date)::integer,fact.property_group,
		  fact.%s::float8,fact.is_quality_eligible,
		  fact.representative_eligible,fact.quality_reasons,evaluation.status,
		  fact.statistical_region_id,fact.municipality_id,fact.city_district_id,
		  fact.local_community_id,fact.settlement_id
		FROM %s fact
		LEFT JOIN %s evaluation USING(transaction_id)
		WHERE fact.transaction_id=$1 AND fact.contract_date IS NOT NULL
		  AND fact.%s>0`, amountColumn, factTable, outlierTable, amountColumn)
	err = tx.QueryRow(ctx, query, id).Scan(
		&subject.year, &subject.propertyGroup, &subject.actual,
		&subject.quality.IsQualityEligible, &subject.quality.IsRepresentative,
		&subject.quality.Reasons, &subject.outlierStatus, &subject.regionID,
		&subject.municipalityID, &subject.cityDistrictID,
		&subject.localCommunityID, &subject.settlementID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransactionContext{}, ErrTransactionNotFound
	}
	if err != nil {
		return TransactionContext{}, fmt.Errorf("load context subject: %w", err)
	}
	result := TransactionContext{
		TransactionKind: kind, TransactionID: id, ContractYear: subject.year,
		IncompletePeriod: subject.year == time.Now().Year(),
		PropertyGroup:    subject.propertyGroup, ActualAmountPerSQM: subject.actual,
		Benchmarks: []ContextBenchmark{}, Quality: subject.quality,
		OutlierStatus:      subject.outlierStatus,
		MethodologyVersion: "same-year-representative-context-v1",
	}

	for _, radius := range []int{500, 1000, 2500, 5000} {
		benchmark, ok, loadErr := loadContextBenchmark(ctx, tx, benchmarkQuery{
			table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
			subjectID: id, actual: subject.actual,
			propertyGroup: subject.propertyGroup, year: subject.year,
			scope: "nearby", label: fmt.Sprintf("Primerljivi posli v krogu %s",
				formatRadius(radius)), radius: radius,
		})
		if loadErr != nil {
			return TransactionContext{}, loadErr
		}
		if ok {
			result.Benchmarks = append(result.Benchmarks, benchmark)
		}
		if ok && benchmark.SampleSize >= 5 {
			break
		}
	}

	geographies := []struct {
		scope, column, label string
		id                   *int64
	}{
		{"city_district", "city_district_id", "Mestna četrt", subject.cityDistrictID},
		{"local_community", "local_community_id", "Krajevna skupnost", subject.localCommunityID},
		{"settlement", "settlement_id", "Naselje", subject.settlementID},
		{"municipality", "municipality_id", "Občina", subject.municipalityID},
		{"statistical_region", "statistical_region_id", "Statistična regija", subject.regionID},
	}
	for _, geography := range geographies {
		if geography.id == nil {
			continue
		}
		benchmark, ok, loadErr := loadContextBenchmark(ctx, tx, benchmarkQuery{
			table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
			subjectID: id, actual: subject.actual,
			propertyGroup: subject.propertyGroup, year: subject.year,
			scope: geography.scope, label: geography.label,
			geographyColumn: geography.column, geographyID: geography.id,
		})
		if loadErr != nil {
			return TransactionContext{}, loadErr
		}
		if ok {
			result.Benchmarks = append(result.Benchmarks, benchmark)
		}
	}
	country, ok, err := loadContextBenchmark(ctx, tx, benchmarkQuery{
		table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
		subjectID: id, actual: subject.actual,
		propertyGroup: subject.propertyGroup, year: subject.year,
		scope: "country", label: "Slovenija",
	})
	if err != nil {
		return TransactionContext{}, err
	}
	if ok {
		result.Benchmarks = append(result.Benchmarks, country)
	}

	valuation, valuationErr := r.ValuationPrediction(ctx, kind, id)
	if valuationErr == nil {
		result.Valuation = &ContextValuation{
			PredictedAmountPerSQM: valuation.PredictedAmountPerSQM,
			LowerAmountPerSQM:     valuation.LowerAmountPerSQM,
			UpperAmountPerSQM:     valuation.UpperAmountPerSQM,
			PredictedTotal:        valuation.PredictedTotal,
			LowerTotal:            valuation.LowerTotal,
			UpperTotal:            valuation.UpperTotal,
			ModelVersion:          valuation.ModelVersion,
			Reliability:           valuation.Reliability,
			Warnings:              valuation.Warnings,
		}
	} else if !errors.Is(valuationErr, ErrPredictionNotFound) {
		return TransactionContext{}, valuationErr
	}
	return result, tx.Commit(ctx)
}

type benchmarkQuery struct {
	table, subjectTable, amountColumn, propertyGroup, scope, label string
	subjectID                                                      int64
	year                                                           int
	actual                                                         float64
	geographyColumn                                                string
	geographyID                                                    *int64
	radius                                                         int
}

func loadContextBenchmark(
	ctx context.Context, tx pgx.Tx, input benchmarkQuery,
) (ContextBenchmark, bool, error) {
	predicate := "TRUE"
	args := []any{input.subjectID, input.propertyGroup, input.year, input.actual}
	if input.geographyID != nil {
		predicate = fmt.Sprintf("fact.%s=$5", input.geographyColumn)
		args = append(args, *input.geographyID)
	} else if input.scope == "nearby" {
		predicate = fmt.Sprintf(`fact.location IS NOT NULL AND ST_DWithin(
		  fact.location,(SELECT location FROM %s WHERE transaction_id=$1),$5
		)`, input.subjectTable)
		args = append(args, input.radius)
	}
	query := fmt.Sprintf(`
		SELECT count(*)::bigint,
		  percentile_cont(.5) WITHIN GROUP(ORDER BY fact.%s)::float8,
		  100.0*count(*) FILTER(WHERE fact.%s<=$4)/NULLIF(count(*),0)::float8
		FROM %s fact
		WHERE fact.transaction_id<>$1
		  AND fact.property_group=$2
		  AND fact.contract_date>=make_date($3,1,1)
		  AND fact.contract_date<make_date($3+1,1,1)
		  AND fact.representative_eligible
		  AND fact.%s>0
		  AND %s`, input.amountColumn, input.amountColumn, input.table,
		input.amountColumn, predicate)
	var sample int64
	var median, percentile *float64
	if err := tx.QueryRow(ctx, query, args...).Scan(
		&sample, &median, &percentile,
	); err != nil {
		return ContextBenchmark{}, false, fmt.Errorf(
			"load %s context benchmark: %w", input.scope, err)
	}
	if sample < 5 || median == nil || percentile == nil {
		return ContextBenchmark{}, false, nil
	}
	difference, valid := benchmarkDifference(input.actual, *median)
	if !valid {
		return ContextBenchmark{}, false, nil
	}
	label := input.label
	if input.geographyID != nil {
		if err := tx.QueryRow(ctx,
			"SELECT name FROM analytics.geographic_units WHERE id=$1",
			*input.geographyID).Scan(&label); err != nil {
			return ContextBenchmark{}, false, fmt.Errorf(
				"load context geography label: %w", err)
		}
	}
	benchmark := ContextBenchmark{
		Scope: input.scope, Label: label, GeographyID: input.geographyID,
		MedianAmountPerSQM: *median, SampleSize: sample,
		DifferencePercent: difference, Percentile: *percentile,
		Period: input.year, Reliability: benchmarkReliability(sample),
		IsPublishable: sample >= descriptivePublicationThreshold,
	}
	if input.scope == "nearby" {
		benchmark.RadiusMeters = &input.radius
	}
	return benchmark, true, nil
}

func formatRadius(radius int) string {
	if radius >= 1000 {
		return fmt.Sprintf("%.1f km", float64(radius)/1000)
	}
	return fmt.Sprintf("%d m", radius)
}
