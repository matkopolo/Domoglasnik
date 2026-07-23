package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrTransactionNotFound = errors.New("transaction not found")

type QualityDecision struct {
	IsQualityEligible bool     `json:"isQualityEligible"`
	IsRepresentative  bool     `json:"isRepresentative"`
	Reasons           []string `json:"reasons"`
}

type OutlierEvidence struct {
	Status         string   `json:"status"`
	Method         *string  `json:"method"`
	CohortLevel    *string  `json:"cohortLevel"`
	CohortSize     *int     `json:"cohortSize"`
	CohortMedian   *float64 `json:"cohortMedian"`
	LowerBound     *float64 `json:"lowerBound"`
	UpperBound     *float64 `json:"upperBound"`
	ModifiedZScore *float64 `json:"modifiedZScore"`
	RuleVersion    string   `json:"ruleVersion"`
}

type TransactionGeography struct {
	ID    int64  `json:"id"`
	Level string `json:"level"`
	Name  string `json:"name"`
}

type SaleBuildingPart struct {
	ID                        int64    `json:"id"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode"`
	BuildingNumber            *string  `json:"buildingNumber"`
	BuildingPartNumber        *string  `json:"buildingPartNumber"`
	BuildingPartType          *int32   `json:"buildingPartType"`
	ConstructionYear          *int32   `json:"constructionYear"`
	SoldArea                  *float64 `json:"soldArea"`
	SoldUsableArea            *float64 `json:"soldUsableArea"`
	ContractualPrice          *float64 `json:"contractualPrice"`
	Settlement                *string  `json:"settlement"`
	Street                    *string  `json:"street"`
	HouseNumber               *string  `json:"houseNumber"`
}

type SaleLandParcel struct {
	ID                        int64    `json:"id"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode"`
	ParcelNumber              *string  `json:"parcelNumber"`
	LandType                  *int32   `json:"landType"`
	ParcelArea                *float64 `json:"parcelArea"`
	ContractualPrice          *float64 `json:"contractualPrice"`
}

type SaleTransactionDetail struct {
	TransactionID   int64                  `json:"transactionId"`
	ContractDate    *string                `json:"contractDate"`
	DatasetYear     int                    `json:"datasetYear"`
	PropertyGroup   string                 `json:"propertyGroup"`
	Area            *float64               `json:"area"`
	Price           *float64               `json:"price"`
	PricePerSQM     *float64               `json:"pricePerSqm"`
	PriceDerivation string                 `json:"priceDerivation"`
	Longitude       *float64               `json:"longitude"`
	Latitude        *float64               `json:"latitude"`
	Quality         QualityDecision        `json:"quality"`
	Outlier         *OutlierEvidence       `json:"outlier"`
	Geographies     []TransactionGeography `json:"geographies"`
	BuildingParts   []SaleBuildingPart     `json:"buildingParts"`
	LandParcels     []SaleLandParcel       `json:"landParcels"`
}

type RentalSpace struct {
	ID                        int64    `json:"id"`
	CadastralMunicipalityCode *int32   `json:"cadastralMunicipalityCode"`
	BuildingNumber            *string  `json:"buildingNumber"`
	BuildingPartNumber        *string  `json:"buildingPartNumber"`
	RentedSpaceType           *int32   `json:"rentedSpaceType"`
	Furnishing                *int32   `json:"furnishing"`
	Area                      *float64 `json:"area"`
	UsableArea                *float64 `json:"usableArea"`
	ContractualRent           *float64 `json:"contractualRent"`
	ConstructionYear          *int32   `json:"constructionYear"`
	Settlement                *string  `json:"settlement"`
	Street                    *string  `json:"street"`
	HouseNumber               *string  `json:"houseNumber"`
}

type RentalTransactionDetail struct {
	TransactionID          int64                  `json:"transactionId"`
	ContractDate           *string                `json:"contractDate"`
	RentalStartDate        *string                `json:"rentalStartDate"`
	RentalEndDate          *string                `json:"rentalEndDate"`
	DatasetYear            int                    `json:"datasetYear"`
	PropertyGroup          string                 `json:"propertyGroup"`
	Area                   *float64               `json:"area"`
	MonthlyRentReported    *float64               `json:"monthlyRentReported"`
	MonthlyRentExVAT       *float64               `json:"monthlyRentExVat"`
	MonthlyRentPerSQM      *float64               `json:"monthlyRentPerSqm"`
	RentDerivation         string                 `json:"rentDerivation"`
	RentalTerm             string                 `json:"rentalTerm"`
	DurationMonths         *int                   `json:"durationMonths"`
	OperatingCostsIncluded *bool                  `json:"operatingCostsIncluded"`
	IsMarketRent           bool                   `json:"isMarketRent"`
	Longitude              *float64               `json:"longitude"`
	Latitude               *float64               `json:"latitude"`
	Quality                QualityDecision        `json:"quality"`
	Outlier                *OutlierEvidence       `json:"outlier"`
	Geographies            []TransactionGeography `json:"geographies"`
	Spaces                 []RentalSpace          `json:"spaces"`
}

type TransactionDetailStore interface {
	SaleTransaction(context.Context, int64) (SaleTransactionDetail, error)
	RentalTransaction(context.Context, int64) (RentalTransactionDetail, error)
}

func (r *StatisticsRepository) SaleTransaction(
	ctx context.Context, id int64,
) (SaleTransactionDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return SaleTransactionDetail{}, fmt.Errorf("begin sale detail: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result SaleTransactionDetail
	var outlier OutlierEvidence
	var outlierStatus *string
	var outlierRuleVersion *string
	err = tx.QueryRow(ctx, `
		SELECT f.transaction_id,f.contract_date::text,f.dataset_year,f.property_group,
		  f.area::float8,f.price::float8,f.price_per_sqm::float8,f.price_derivation,
		  ST_X(ST_Transform(f.location,4326)),ST_Y(ST_Transform(f.location,4326)),
		  f.is_quality_eligible,f.representative_eligible,f.quality_reasons,
		  e.status,e.method,e.cohort_level,e.cohort_size,e.cohort_median::float8,
		  e.lower_bound::float8,e.upper_bound::float8,e.modified_z_score::float8,
		  e.rule_version
		FROM analytics.sale_facts f
		LEFT JOIN analytics.sale_outlier_evaluations e USING(transaction_id)
		WHERE f.transaction_id=$1`, id).Scan(
		&result.TransactionID, &result.ContractDate, &result.DatasetYear,
		&result.PropertyGroup, &result.Area, &result.Price, &result.PricePerSQM,
		&result.PriceDerivation, &result.Longitude, &result.Latitude,
		&result.Quality.IsQualityEligible, &result.Quality.IsRepresentative,
		&result.Quality.Reasons, &outlierStatus, &outlier.Method,
		&outlier.CohortLevel, &outlier.CohortSize, &outlier.CohortMedian,
		&outlier.LowerBound, &outlier.UpperBound, &outlier.ModifiedZScore,
		&outlierRuleVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return SaleTransactionDetail{}, ErrTransactionNotFound
	}
	if err != nil {
		return SaleTransactionDetail{}, fmt.Errorf("load sale detail: %w", err)
	}
	if outlierStatus != nil {
		outlier.Status = *outlierStatus
		outlier.RuleVersion = *outlierRuleVersion
		result.Outlier = &outlier
	}
	if err := loadGeographies(ctx, tx, []any{
		id, "analytics.sale_facts",
	}, &result.Geographies); err != nil {
		return SaleTransactionDetail{}, err
	}
	result.BuildingParts, err = loadSaleBuildingParts(ctx, tx, id)
	if err != nil {
		return SaleTransactionDetail{}, err
	}
	result.LandParcels, err = loadSaleLandParcels(ctx, tx, id)
	if err != nil {
		return SaleTransactionDetail{}, err
	}
	return result, tx.Commit(ctx)
}

func (r *StatisticsRepository) RentalTransaction(
	ctx context.Context, id int64,
) (RentalTransactionDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return RentalTransactionDetail{}, fmt.Errorf("begin rental detail: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result RentalTransactionDetail
	var outlier OutlierEvidence
	var outlierStatus *string
	var outlierRuleVersion *string
	err = tx.QueryRow(ctx, `
		SELECT f.transaction_id,f.contract_date::text,f.rental_start_date::text,
		  f.rental_end_date::text,f.dataset_year,f.property_group,f.area::float8,
		  f.monthly_rent_reported::float8,f.monthly_rent_ex_vat::float8,
		  f.monthly_rent_per_sqm::float8,f.rent_derivation,f.rental_term,
		  f.duration_months,f.operating_costs_included,f.is_market_rent,
		  ST_X(ST_Transform(f.location,4326)),ST_Y(ST_Transform(f.location,4326)),
		  f.is_quality_eligible,f.representative_eligible,f.quality_reasons,
		  e.status,e.method,e.cohort_level,e.cohort_size,e.cohort_median::float8,
		  e.lower_bound::float8,e.upper_bound::float8,e.modified_z_score::float8,
		  e.rule_version
		FROM analytics.rental_facts f
		LEFT JOIN analytics.rental_outlier_evaluations e USING(transaction_id)
		WHERE f.transaction_id=$1`, id).Scan(
		&result.TransactionID, &result.ContractDate, &result.RentalStartDate,
		&result.RentalEndDate, &result.DatasetYear, &result.PropertyGroup,
		&result.Area, &result.MonthlyRentReported, &result.MonthlyRentExVAT,
		&result.MonthlyRentPerSQM, &result.RentDerivation, &result.RentalTerm,
		&result.DurationMonths, &result.OperatingCostsIncluded,
		&result.IsMarketRent, &result.Longitude, &result.Latitude,
		&result.Quality.IsQualityEligible, &result.Quality.IsRepresentative,
		&result.Quality.Reasons, &outlierStatus, &outlier.Method,
		&outlier.CohortLevel, &outlier.CohortSize, &outlier.CohortMedian,
		&outlier.LowerBound, &outlier.UpperBound, &outlier.ModifiedZScore,
		&outlierRuleVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return RentalTransactionDetail{}, ErrTransactionNotFound
	}
	if err != nil {
		return RentalTransactionDetail{}, fmt.Errorf("load rental detail: %w", err)
	}
	if outlierStatus != nil {
		outlier.Status = *outlierStatus
		outlier.RuleVersion = *outlierRuleVersion
		result.Outlier = &outlier
	}
	if err := loadGeographies(ctx, tx, []any{
		id, "analytics.rental_facts",
	}, &result.Geographies); err != nil {
		return RentalTransactionDetail{}, err
	}
	result.Spaces, err = loadRentalSpaces(ctx, tx, id)
	if err != nil {
		return RentalTransactionDetail{}, err
	}
	return result, tx.Commit(ctx)
}

func loadGeographies(
	ctx context.Context, tx pgx.Tx, args []any, destination *[]TransactionGeography,
) error {
	// The table name is selected internally, never from an HTTP parameter.
	table := args[1].(string)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT unit.id,unit.level,unit.name
		FROM %s fact
		CROSS JOIN LATERAL unnest(ARRAY[
		  fact.statistical_region_id,fact.municipality_id,fact.city_district_id,
		  fact.local_community_id,fact.settlement_id]) geography_id
		JOIN analytics.geographic_units unit ON unit.id=geography_id
		WHERE fact.transaction_id=$1
		ORDER BY array_position(ARRAY[
		  'statistical_region','municipality','city_district',
		  'local_community','settlement'],unit.level)`, table), args[0])
	if err != nil {
		return fmt.Errorf("load transaction geographies: %w", err)
	}
	defer rows.Close()
	*destination = make([]TransactionGeography, 0)
	for rows.Next() {
		var item TransactionGeography
		if err := rows.Scan(&item.ID, &item.Level, &item.Name); err != nil {
			return fmt.Errorf("scan transaction geography: %w", err)
		}
		*destination = append(*destination, item)
	}
	return rows.Err()
}

func loadSaleBuildingParts(ctx context.Context, tx pgx.Tx, id int64) ([]SaleBuildingPart, error) {
	rows, err := tx.Query(ctx, `
		SELECT id,cadastral_municipality_code,building_number,building_part_number,
		  building_part_type,construction_year,sold_area::float8,
		  sold_usable_area::float8,contractual_price::float8,settlement,street,house_number
		FROM building_parts WHERE transaction_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("load sale building parts: %w", err)
	}
	defer rows.Close()
	items := make([]SaleBuildingPart, 0)
	for rows.Next() {
		var item SaleBuildingPart
		if err := rows.Scan(&item.ID, &item.CadastralMunicipalityCode,
			&item.BuildingNumber, &item.BuildingPartNumber, &item.BuildingPartType,
			&item.ConstructionYear, &item.SoldArea, &item.SoldUsableArea,
			&item.ContractualPrice, &item.Settlement, &item.Street,
			&item.HouseNumber); err != nil {
			return nil, fmt.Errorf("scan sale building part: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadSaleLandParcels(ctx context.Context, tx pgx.Tx, id int64) ([]SaleLandParcel, error) {
	rows, err := tx.Query(ctx, `
		SELECT id,cadastral_municipality_code,parcel_number,land_type,
		  parcel_area::float8,contractual_price::float8
		FROM land_parcels WHERE transaction_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("load sale land parcels: %w", err)
	}
	defer rows.Close()
	items := make([]SaleLandParcel, 0)
	for rows.Next() {
		var item SaleLandParcel
		if err := rows.Scan(&item.ID, &item.CadastralMunicipalityCode,
			&item.ParcelNumber, &item.LandType, &item.ParcelArea,
			&item.ContractualPrice); err != nil {
			return nil, fmt.Errorf("scan sale land parcel: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadRentalSpaces(ctx context.Context, tx pgx.Tx, id int64) ([]RentalSpace, error) {
	rows, err := tx.Query(ctx, `
		SELECT id,cadastral_municipality_code,building_number,building_part_number,
		  rented_spaces_type,furnishing,rented_spaces_area::float8,
		  rented_usable_area::float8,contractual_rent::float8,construction_year,
		  settlement,street,house_number
		FROM rental_building_parts WHERE transaction_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("load rental spaces: %w", err)
	}
	defer rows.Close()
	items := make([]RentalSpace, 0)
	for rows.Next() {
		var item RentalSpace
		if err := rows.Scan(&item.ID, &item.CadastralMunicipalityCode,
			&item.BuildingNumber, &item.BuildingPartNumber, &item.RentedSpaceType,
			&item.Furnishing, &item.Area, &item.UsableArea, &item.ContractualRent,
			&item.ConstructionYear, &item.Settlement, &item.Street,
			&item.HouseNumber); err != nil {
			return nil, fmt.Errorf("scan rental space: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
