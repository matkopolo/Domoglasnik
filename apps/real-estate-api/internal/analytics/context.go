package analytics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
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
	ComparisonProfile  string  `json:"comparisonProfile"`
	AreaTolerancePct   *int    `json:"areaTolerancePercent,omitempty"`
	BuildingAgeYears   *int    `json:"constructionYearTolerance,omitempty"`
}

type DirectComparable struct {
	TransactionID                 int64   `json:"transactionId"`
	ContractDate                  string  `json:"contractDate"`
	ContractYear                  int     `json:"contractYear"`
	Area                          float64 `json:"area"`
	Amount                        float64 `json:"amount"`
	AmountPerSQM                  float64 `json:"amountPerSqm"`
	AreaDifferencePercent         float64 `json:"areaDifferencePercent"`
	AmountPerSQMDifferencePercent float64 `json:"amountPerSqmDifferencePercent"`
	YearDifference                int     `json:"yearDifference"`
	MatchTier                     string  `json:"matchTier"`
	ConstructionYear              *int    `json:"constructionYear,omitempty"`
	BuildingPartNumber            *string `json:"buildingPartNumber,omitempty"`
	IsRepresentative              bool    `json:"isRepresentative"`
	OutlierStatus                 *string `json:"outlierStatus,omitempty"`
}

type TransactionContext struct {
	TransactionKind    string                 `json:"transactionKind"`
	TransactionID      int64                  `json:"transactionId"`
	ContractYear       int                    `json:"contractYear"`
	IncompletePeriod   bool                   `json:"incompletePeriod"`
	PropertyGroup      string                 `json:"propertyGroup"`
	ActualAmountPerSQM float64                `json:"actualAmountPerSqm"`
	DirectComparables  []DirectComparable     `json:"directComparables"`
	Benchmarks         []ContextBenchmark     `json:"benchmarks"`
	Valuation          *ContextValuation      `json:"valuation"`
	CurrentMarket      *CurrentMarketEstimate `json:"currentMarketEstimate,omitempty"`
	Quality            QualityDecision        `json:"quality"`
	OutlierStatus      *string                `json:"outlierStatus"`
	MethodologyVersion string                 `json:"methodologyVersion"`
}

type CurrentMarketEstimate struct {
	LowerTotal                float64  `json:"lowerTotal"`
	CentralTotal              float64  `json:"centralTotal"`
	UpperTotal                float64  `json:"upperTotal"`
	UnadjustedLowerTotal      float64  `json:"unadjustedLowerTotal"`
	UnadjustedCentralTotal    float64  `json:"unadjustedCentralTotal"`
	UnadjustedUpperTotal      float64  `json:"unadjustedUpperTotal"`
	LowerAmountPerSQM         float64  `json:"lowerAmountPerSqm"`
	CentralAmountPerSQM       float64  `json:"centralAmountPerSqm"`
	UpperAmountPerSQM         float64  `json:"upperAmountPerSqm"`
	UnadjustedLowerPerSQM     float64  `json:"unadjustedLowerAmountPerSqm"`
	UnadjustedCentralPerSQM   float64  `json:"unadjustedCentralAmountPerSqm"`
	UnadjustedUpperPerSQM     float64  `json:"unadjustedUpperAmountPerSqm"`
	ComparableCount           int      `json:"comparableCount"`
	MaximumRadiusMeters       int      `json:"maximumRadiusMeters"`
	AreaTolerancePercent      int      `json:"areaTolerancePercent"`
	ConstructionYearTolerance *int     `json:"constructionYearTolerance,omitempty"`
	LocationAdjustmentFactor  float64  `json:"locationAdjustmentFactor"`
	RawLocationFactor         float64  `json:"rawLocationAdjustmentFactor"`
	LocationAdjustmentApplied bool     `json:"locationAdjustmentApplied"`
	LocalAdjustmentSampleSize int      `json:"localAdjustmentSampleSize"`
	PackageCompositionMatched bool     `json:"packageCompositionMatched"`
	ReferenceYear             int      `json:"referenceYear"`
	Confidence                string   `json:"confidence"`
	Warnings                  []string `json:"warnings"`
	MethodologyVersion        string   `json:"methodologyVersion"`
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
	area             float64
	cadastralCode    *int32
	buildingNumber   *string
	constructionYear *int
	packageCount     int
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
	packageExpression := "fact.building_part_count+fact.land_parcel_count"
	if kind == "rent" {
		packageExpression = "fact.space_count"
	}
	query := fmt.Sprintf(`
		SELECT extract(year FROM fact.contract_date)::integer,fact.property_group,
		  fact.%s::float8,fact.is_quality_eligible,
		  fact.representative_eligible,fact.quality_reasons,evaluation.status,
		  fact.statistical_region_id,fact.municipality_id,fact.city_district_id,
		  fact.local_community_id,fact.settlement_id,fact.area::float8,
		  fact.cadastral_municipality_code,fact.building_number,
		  fact.construction_year,%s
		FROM %s fact
		LEFT JOIN %s evaluation USING(transaction_id)
		WHERE fact.transaction_id=$1 AND fact.contract_date IS NOT NULL
		  AND fact.%s>0`, amountColumn, packageExpression, factTable, outlierTable, amountColumn)
	err = tx.QueryRow(ctx, query, id).Scan(
		&subject.year, &subject.propertyGroup, &subject.actual,
		&subject.quality.IsQualityEligible, &subject.quality.IsRepresentative,
		&subject.quality.Reasons, &subject.outlierStatus, &subject.regionID,
		&subject.municipalityID, &subject.cityDistrictID,
		&subject.localCommunityID, &subject.settlementID, &subject.area,
		&subject.cadastralCode, &subject.buildingNumber, &subject.constructionYear,
		&subject.packageCount,
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
		DirectComparables: []DirectComparable{}, Benchmarks: []ContextBenchmark{},
		Quality:            subject.quality,
		OutlierStatus:      subject.outlierStatus,
		MethodologyVersion: "physical-similarity-context-v3",
	}

	if subject.cadastralCode != nil && subject.buildingNumber != nil {
		direct, loadErr := loadDirectComparables(ctx, tx, directComparableQuery{
			table: factTable, amountColumn: amountColumn,
			totalColumn:  map[string]string{"sale": "price", "rent": "monthly_rent_ex_vat"}[kind],
			outlierTable: outlierTable, subjectID: id, actual: subject.actual,
			area: subject.area, propertyGroup: subject.propertyGroup, year: subject.year,
			cadastralCode: *subject.cadastralCode, buildingNumber: *subject.buildingNumber,
		})
		if loadErr != nil {
			return TransactionContext{}, loadErr
		}
		result.DirectComparables = direct
	}

	nearbyFound := false
	for _, profile := range nearbyBenchmarkProfiles(subject.constructionYear) {
		for _, radius := range []int{500, 1000, 2500, 5000} {
			benchmark, ok, loadErr := loadContextBenchmark(ctx, tx, benchmarkQuery{
				table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
				subjectID: id, actual: subject.actual, subjectArea: subject.area,
				subjectConstructionYear: subject.constructionYear, profile: profile,
				propertyGroup: subject.propertyGroup, year: subject.year,
				scope: "nearby", label: fmt.Sprintf("Primerljivi posli v krogu %s",
					formatRadius(radius)), radius: radius,
			})
			if loadErr != nil {
				return TransactionContext{}, loadErr
			}
			if ok {
				result.Benchmarks = append(result.Benchmarks, benchmark)
				nearbyFound = true
				break
			}
		}
		if nearbyFound {
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
		profile := unadjustedBenchmarkProfile()
		if geography.scope != "municipality" && geography.scope != "statistical_region" {
			profile = matchedGeographyProfile(subject.constructionYear)
		}
		benchmark, ok, loadErr := loadContextBenchmark(ctx, tx, benchmarkQuery{
			table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
			subjectID: id, actual: subject.actual, subjectArea: subject.area,
			subjectConstructionYear: subject.constructionYear, profile: profile,
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
		subjectArea: subject.area, subjectConstructionYear: subject.constructionYear,
		profile:       unadjustedBenchmarkProfile(),
		propertyGroup: subject.propertyGroup, year: subject.year,
		scope: "country", label: "Slovenija",
	})
	if err != nil {
		return TransactionContext{}, err
	}
	if ok {
		result.Benchmarks = append(result.Benchmarks, country)
	}

	currentMarket, estimateErr := loadCurrentMarketEstimate(ctx, tx, currentMarketQuery{
		table: cohortTable, subjectTable: factTable, amountColumn: amountColumn,
		packageColumn: map[string]string{
			"sale": "building_part_count+land_parcel_count", "rent": "space_count",
		}[kind],
		subjectID: id, propertyGroup: subject.propertyGroup, area: subject.area,
		constructionYear: subject.constructionYear, packageCount: subject.packageCount,
		municipalityID: subject.municipalityID,
	})
	if estimateErr != nil {
		return TransactionContext{}, estimateErr
	}
	result.CurrentMarket = currentMarket

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

type currentMarketQuery struct {
	table, subjectTable, amountColumn, packageColumn, propertyGroup string
	subjectID                                                       int64
	area                                                            float64
	constructionYear                                                *int
	packageCount                                                    int
	municipalityID                                                  *int64
}

type marketCandidate struct {
	amountPerSQM, area, distance float64
	constructionYear             *int
	contractDate                 time.Time
	packageCount                 int
	municipalityID               *int64
}

const (
	currentMarketMinimumSample = 5
	currentMarketTargetSample  = 50
	locationMinimumLocalSample = 20
	locationMinimumTotalSample = 50
	locationShrinkageStrength  = 30.0
)

func loadCurrentMarketEstimate(
	ctx context.Context, tx pgx.Tx, input currentMarketQuery,
) (*CurrentMarketEstimate, error) {
	if input.area <= 0 {
		return nil, nil
	}
	profiles := nearbyBenchmarkProfiles(input.constructionYear)
	radii := []int{1000, 2500, 5000, 10000, 20000}
	var selected []marketCandidate
	var selectedProfile benchmarkProfile
	selectedRadius := 0
	selectedStartYear := time.Now().Year()
	packageCompositionMatched := true
	search := func(requirePackageMatch bool) error {
		targetReached := false
		for _, startYear := range []int{time.Now().Year(), time.Now().Year() - 1} {
			for _, profile := range profiles {
				for _, radius := range radii {
					candidates, err := queryCurrentCandidates(ctx, tx, input, profile, radius, startYear, requirePackageMatch)
					if err != nil {
						return err
					}
					if len(candidates) > len(selected) {
						selected, selectedProfile, selectedRadius, selectedStartYear = candidates, profile, radius, startYear
					}
					if len(candidates) >= currentMarketTargetSample {
						selected, selectedProfile, selectedRadius, selectedStartYear = candidates, profile, radius, startYear
						targetReached = true
						break
					}
				}
				if targetReached {
					break
				}
			}
			if targetReached {
				break
			}
		}
		return nil
	}
	if err := search(true); err != nil {
		return nil, err
	}
	if len(selected) < currentMarketMinimumSample {
		selected = nil
		packageCompositionMatched = false
		if err := search(false); err != nil {
			return nil, err
		}
	}
	if len(selected) < currentMarketMinimumSample {
		return nil, nil
	}

	weighted := make([]weightedValue, 0, len(selected))
	localCurrent := 0
	for _, candidate := range selected {
		if input.municipalityID != nil && candidate.municipalityID != nil &&
			*input.municipalityID == *candidate.municipalityID {
			localCurrent++
		}
		areaTolerance := float64(selectedProfile.areaTolerancePercent) / 100
		areaScore := math.Max(0, 1-math.Abs(candidate.area/input.area-1)/areaTolerance)
		ageScore := .5
		if input.constructionYear != nil && candidate.constructionYear != nil &&
			selectedProfile.constructionYearTolerance > 0 {
			ageScore = math.Max(0, 1-math.Abs(float64(*candidate.constructionYear-*input.constructionYear))/
				float64(selectedProfile.constructionYearTolerance))
		}
		distanceScore := math.Max(0, 1-candidate.distance/20000)
		recencyScore := math.Max(0, 1-time.Since(candidate.contractDate).Hours()/(24*730))
		packageScore := 0.0
		if candidate.packageCount == input.packageCount {
			packageScore = 1
		}
		weight := 35*areaScore + 25*ageScore + 20*distanceScore + 10*recencyScore + 10*packageScore
		weighted = append(weighted, weightedValue{candidate.amountPerSQM, math.Max(weight, 1)})
	}

	locationFactor, rawLocationFactor, localSample, adjustmentErr := currentMarketLocationFactor(
		ctx, tx, input, selectedProfile, selectedRadius, localCurrent,
	)
	if adjustmentErr != nil {
		return nil, adjustmentErr
	}
	unadjustedLower := weightedQuantile(weighted, .25)
	unadjustedCentral := weightedQuantile(weighted, .5)
	unadjustedUpper := weightedQuantile(weighted, .75)
	lower := unadjustedLower * locationFactor
	central := unadjustedCentral * locationFactor
	upper := unadjustedUpper * locationFactor
	warnings := []string{"statistična tržna ocena, ne uradna cenitev", "tekoče leto še ni zaključeno"}
	if selectedStartYear < time.Now().Year() {
		warnings = append(warnings, "za doseganje ciljnega vzorca so vključeni tudi posli prejšnjega koledarskega leta")
	}
	if len(selected) < currentMarketTargetSample {
		warnings = append(warnings, "cilj 50 primerljivih poslov ni bil dosežen znotraj največ 20 km")
	}
	if locationFactor != 1 {
		warnings = append(warnings, "uporabljena zgodovinska prilagoditev med domačo občino in okoliškim trgom")
	} else if rawLocationFactor != 1 {
		warnings = append(warnings, "lokalna prilagoditev ni uporabljena zaradi premajhnega podpornega vzorca")
	}
	if !packageCompositionMatched {
		warnings = append(warnings, "premalo poslov z enakim številom sestavin paketa; uporabljena je mešana sestava paketov")
	}
	confidence := "very_low"
	if len(selected) >= 20 {
		confidence = "low"
	}
	if len(selected) >= currentMarketTargetSample && selectedRadius <= 10000 && selectedProfile.areaTolerancePercent == 15 {
		confidence = "moderate"
	}
	var ageTolerance *int
	if selectedProfile.constructionYearTolerance > 0 {
		ageTolerance = &selectedProfile.constructionYearTolerance
	}
	return &CurrentMarketEstimate{
		LowerTotal: lower * input.area, CentralTotal: central * input.area, UpperTotal: upper * input.area,
		UnadjustedLowerTotal: unadjustedLower * input.area, UnadjustedCentralTotal: unadjustedCentral * input.area,
		UnadjustedUpperTotal: unadjustedUpper * input.area,
		LowerAmountPerSQM:    lower, CentralAmountPerSQM: central, UpperAmountPerSQM: upper,
		UnadjustedLowerPerSQM: unadjustedLower, UnadjustedCentralPerSQM: unadjustedCentral,
		UnadjustedUpperPerSQM: unadjustedUpper,
		ComparableCount:       len(selected), MaximumRadiusMeters: selectedRadius,
		AreaTolerancePercent:      selectedProfile.areaTolerancePercent,
		ConstructionYearTolerance: ageTolerance, LocationAdjustmentFactor: locationFactor,
		RawLocationFactor: rawLocationFactor, LocationAdjustmentApplied: locationFactor != 1,
		LocalAdjustmentSampleSize: localSample, PackageCompositionMatched: packageCompositionMatched,
		ReferenceYear: time.Now().Year(),
		Confidence:    confidence, Warnings: warnings,
		MethodologyVersion: "adaptive-current-market-v4",
	}, nil
}

func queryCurrentCandidates(
	ctx context.Context, tx pgx.Tx, input currentMarketQuery, profile benchmarkProfile, radius, startYear int,
	requirePackageMatch bool,
) ([]marketCandidate, error) {
	args := []any{input.subjectID, input.propertyGroup, startYear, input.area, radius, time.Now().Year() + 1}
	ageClause := "TRUE"
	packageClause := "TRUE"
	if requirePackageMatch {
		args = append(args, input.packageCount)
		packageClause = fmt.Sprintf("(%s)=$%d", input.packageColumn, len(args))
	}
	if profile.constructionYearTolerance > 0 && input.constructionYear != nil {
		args = append(args, *input.constructionYear)
		ageClause = fmt.Sprintf("fact.construction_year IS NOT NULL AND abs(fact.construction_year-$%d)<=%d",
			len(args), profile.constructionYearTolerance)
	}
	query := fmt.Sprintf(`SELECT fact.%s::float8,fact.area::float8,
	  ST_Distance(fact.location,subject.location)::float8,fact.construction_year,
	  fact.contract_date,(%s)::integer,fact.municipality_id
	FROM %s fact CROSS JOIN (SELECT location FROM %s WHERE transaction_id=$1) subject
	WHERE fact.transaction_id<>$1 AND fact.property_group=$2
	  AND fact.contract_date>=make_date($3,1,1) AND fact.contract_date<make_date($6,1,1)
	  AND fact.representative_eligible AND fact.%s>0 AND fact.area>0
	  AND subject.location IS NOT NULL AND fact.location IS NOT NULL
	  AND ST_DWithin(fact.location,subject.location,$5)
	  AND abs(fact.area/$4-1)<=%f AND %s AND %s
	ORDER BY ST_Distance(fact.location,subject.location),fact.transaction_id LIMIT 250`,
		input.amountColumn, input.packageColumn, input.table, input.subjectTable,
		input.amountColumn, float64(profile.areaTolerancePercent)/100, ageClause, packageClause)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load current market candidates: %w", err)
	}
	defer rows.Close()
	result := []marketCandidate{}
	for rows.Next() {
		var candidate marketCandidate
		if err := rows.Scan(&candidate.amountPerSQM, &candidate.area, &candidate.distance,
			&candidate.constructionYear, &candidate.contractDate, &candidate.packageCount,
			&candidate.municipalityID); err != nil {
			return nil, fmt.Errorf("scan current market candidate: %w", err)
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func currentMarketLocationFactor(
	ctx context.Context, tx pgx.Tx, input currentMarketQuery, profile benchmarkProfile,
	radius, localCurrent int,
) (float64, float64, int, error) {
	if input.municipalityID == nil || localCurrent >= 5 {
		return 1, 1, localCurrent, nil
	}
	args := []any{input.subjectID, input.propertyGroup, time.Now().Year() - 3,
		time.Now().Year(), input.area, radius, *input.municipalityID}
	ageClause := "TRUE"
	if profile.constructionYearTolerance > 0 && input.constructionYear != nil {
		args = append(args, *input.constructionYear)
		ageClause = fmt.Sprintf("fact.construction_year IS NOT NULL AND abs(fact.construction_year-$%d)<=%d",
			len(args), profile.constructionYearTolerance)
	}
	query := fmt.Sprintf(`SELECT count(*) FILTER(WHERE fact.municipality_id=$7)::integer,
	  count(*)::integer,
	  percentile_cont(.5) WITHIN GROUP(ORDER BY fact.%s) FILTER(WHERE fact.municipality_id=$7)::float8,
	  percentile_cont(.5) WITHIN GROUP(ORDER BY fact.%s)::float8
	FROM %s fact CROSS JOIN (SELECT location FROM %s WHERE transaction_id=$1) subject
	WHERE fact.property_group=$2 AND extract(year FROM fact.contract_date)>=$3
	  AND extract(year FROM fact.contract_date)<$4 AND fact.representative_eligible
	  AND fact.%s>0 AND fact.area>0 AND fact.location IS NOT NULL
	  AND ST_DWithin(fact.location,subject.location,$6)
	  AND abs(fact.area/$5-1)<=%f AND %s`, input.amountColumn, input.amountColumn,
		input.table, input.subjectTable, input.amountColumn,
		float64(profile.areaTolerancePercent)/100, ageClause)
	var localN, allN int
	var localMedian, allMedian *float64
	if err := tx.QueryRow(ctx, query, args...).Scan(&localN, &allN, &localMedian, &allMedian); err != nil {
		return 1, 1, 0, fmt.Errorf("load current market location adjustment: %w", err)
	}
	if localMedian == nil || allMedian == nil || *allMedian <= 0 {
		return 1, 1, localN, nil
	}
	rawFactor := math.Max(.65, math.Min(1.35, *localMedian / *allMedian))
	if localN < locationMinimumLocalSample || allN < locationMinimumTotalSample {
		return 1, rawFactor, localN, nil
	}
	return shrinkLocationFactor(rawFactor, localN), rawFactor, localN, nil
}

func shrinkLocationFactor(rawFactor float64, localSample int) float64 {
	weight := float64(localSample) / (float64(localSample) + locationShrinkageStrength)
	shrunkFactor := 1 + weight*(rawFactor-1)
	return math.Max(.8, math.Min(1.2, shrunkFactor))
}

type weightedValue struct{ value, weight float64 }

func weightedQuantile(values []weightedValue, quantile float64) float64 {
	sort.Slice(values, func(i, j int) bool { return values[i].value < values[j].value })
	var total float64
	for _, value := range values {
		total += value.weight
	}
	target, cumulative := total*quantile, 0.0
	for _, value := range values {
		cumulative += value.weight
		if cumulative >= target {
			return value.value
		}
	}
	return values[len(values)-1].value
}

type directComparableQuery struct {
	table, amountColumn, totalColumn, outlierTable, propertyGroup, buildingNumber string
	subjectID                                                                     int64
	actual, area                                                                  float64
	year                                                                          int
	cadastralCode                                                                 int32
}

func loadDirectComparables(
	ctx context.Context, tx pgx.Tx, input directComparableQuery,
) ([]DirectComparable, error) {
	query := fmt.Sprintf(`
		SELECT fact.transaction_id,fact.contract_date::text,
		  extract(year FROM fact.contract_date)::integer,fact.area::float8,
		  fact.%s::float8,fact.%s::float8,
		  100.0*(fact.area/$4-1)::float8,
		  100.0*(fact.%s/$3-1)::float8,
		  abs(extract(year FROM fact.contract_date)::integer-$5),
		  CASE
		    WHEN extract(year FROM fact.contract_date)::integer=$5
		      AND abs(fact.area/$4-1)<=.15 THEN 1
		    WHEN abs(extract(year FROM fact.contract_date)::integer-$5)=1
		      AND abs(fact.area/$4-1)<=.15 THEN 2
		    WHEN extract(year FROM fact.contract_date)::integer=$5 THEN 3
		    ELSE 99
		  END match_tier,
		  fact.construction_year,fact.building_part_number,
		  fact.representative_eligible,evaluation.status
		FROM %s fact
		LEFT JOIN %s evaluation USING(transaction_id)
		WHERE fact.transaction_id<>$1 AND fact.property_group=$2
		  AND fact.cadastral_municipality_code=$6 AND fact.building_number=$7
		  AND fact.contract_date>=make_date($5-1,1,1)
		  AND fact.contract_date<make_date($5+2,1,1)
		  AND fact.is_quality_eligible AND fact.area>0 AND fact.%s>0
		  AND fact.%s>0
		  AND (extract(year FROM fact.contract_date)::integer=$5
		    OR abs(fact.area/$4-1)<=.15)
		ORDER BY match_tier,abs(fact.area/$4-1),
		  abs(fact.contract_date-make_date($5,7,1)),fact.transaction_id
		LIMIT 8`, input.totalColumn, input.amountColumn, input.amountColumn,
		input.table, input.outlierTable, input.amountColumn, input.totalColumn)
	rows, err := tx.Query(ctx, query, input.subjectID, input.propertyGroup,
		input.actual, input.area, input.year, input.cadastralCode, input.buildingNumber)
	if err != nil {
		return nil, fmt.Errorf("load direct comparables: %w", err)
	}
	defer rows.Close()
	comparables := []DirectComparable{}
	for rows.Next() {
		var comparable DirectComparable
		var tier int
		if err := rows.Scan(
			&comparable.TransactionID, &comparable.ContractDate,
			&comparable.ContractYear, &comparable.Area, &comparable.Amount,
			&comparable.AmountPerSQM, &comparable.AreaDifferencePercent,
			&comparable.AmountPerSQMDifferencePercent, &comparable.YearDifference,
			&tier, &comparable.ConstructionYear, &comparable.BuildingPartNumber,
			&comparable.IsRepresentative, &comparable.OutlierStatus,
		); err != nil {
			return nil, fmt.Errorf("scan direct comparable: %w", err)
		}
		comparable.MatchTier = directComparableTier(tier)
		comparables = append(comparables, comparable)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate direct comparables: %w", err)
	}
	return comparables, nil
}

func directComparableTier(tier int) string {
	switch tier {
	case 1:
		return "same_building_same_year_area_15"
	case 2:
		return "same_building_adjacent_year_area_15"
	case 3:
		return "same_building_same_year"
	default:
		return "same_building_other"
	}
}

type benchmarkProfile struct {
	name                      string
	areaTolerancePercent      int
	constructionYearTolerance int
}

func nearbyBenchmarkProfiles(constructionYear *int) []benchmarkProfile {
	if constructionYear == nil {
		return []benchmarkProfile{{name: "area_15", areaTolerancePercent: 15}}
	}
	return []benchmarkProfile{
		{name: "area_15_age_8", areaTolerancePercent: 15, constructionYearTolerance: 8},
	}
}

func matchedGeographyProfile(constructionYear *int) benchmarkProfile {
	if constructionYear == nil {
		return benchmarkProfile{name: "area_15", areaTolerancePercent: 15}
	}
	return benchmarkProfile{name: "area_15_age_8", areaTolerancePercent: 15, constructionYearTolerance: 8}
}

func unadjustedBenchmarkProfile() benchmarkProfile {
	return benchmarkProfile{name: "unadjusted"}
}

type benchmarkQuery struct {
	table, subjectTable, amountColumn, propertyGroup, scope, label string
	subjectID                                                      int64
	year                                                           int
	actual                                                         float64
	geographyColumn                                                string
	geographyID                                                    *int64
	radius                                                         int
	subjectArea                                                    float64
	subjectConstructionYear                                        *int
	profile                                                        benchmarkProfile
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
	if input.profile.areaTolerancePercent > 0 && input.subjectArea > 0 {
		args = append(args, input.subjectArea)
		predicate += fmt.Sprintf(" AND abs(fact.area/$%d-1)<=%f", len(args),
			float64(input.profile.areaTolerancePercent)/100)
	}
	if input.profile.constructionYearTolerance > 0 && input.subjectConstructionYear != nil {
		args = append(args, *input.subjectConstructionYear)
		predicate += fmt.Sprintf(" AND fact.construction_year IS NOT NULL AND abs(fact.construction_year-$%d)<=%d",
			len(args), input.profile.constructionYearTolerance)
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
		IsPublishable:     sample >= descriptivePublicationThreshold,
		ComparisonProfile: input.profile.name,
	}
	if input.profile.areaTolerancePercent > 0 {
		benchmark.AreaTolerancePct = &input.profile.areaTolerancePercent
	}
	if input.profile.constructionYearTolerance > 0 {
		benchmark.BuildingAgeYears = &input.profile.constructionYearTolerance
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
