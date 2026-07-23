package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrBuildingNotFound = errors.New("building not found")

type BuildingMarketSummary struct {
	TransactionCount    int64    `json:"transactionCount"`
	RepresentativeCount int64    `json:"representativeCount"`
	MedianAmount        *float64 `json:"medianAmount"`
	MedianAmountPerSQM  *float64 `json:"medianAmountPerSqm"`
	FirstContractDate   *string  `json:"firstContractDate"`
	LastContractDate    *string  `json:"lastContractDate"`
}

type BuildingYearPoint struct {
	Year                int      `json:"year"`
	TransactionCount    int64    `json:"transactionCount"`
	RepresentativeCount int64    `json:"representativeCount"`
	MedianAmountPerSQM  *float64 `json:"medianAmountPerSqm"`
}

type BuildingStatistics struct {
	CadastralMunicipalityCode int32                 `json:"cadastralMunicipalityCode"`
	BuildingNumber            string                `json:"buildingNumber"`
	ConstructionYear          *int32                `json:"constructionYear"`
	KnownBuildingPartCount    int64                 `json:"knownBuildingPartCount"`
	Longitude                 *float64              `json:"longitude"`
	Latitude                  *float64              `json:"latitude"`
	Sales                     BuildingMarketSummary `json:"sales"`
	Rentals                   BuildingMarketSummary `json:"rentals"`
	SaleHistory               []BuildingYearPoint   `json:"saleHistory"`
	RentalHistory             []BuildingYearPoint   `json:"rentalHistory"`
}

type BuildingStore interface {
	BuildingStatistics(context.Context, int32, string) (BuildingStatistics, error)
}

func (r *StatisticsRepository) BuildingStatistics(
	ctx context.Context, cadastralCode int32, buildingNumber string,
) (BuildingStatistics, error) {
	result := BuildingStatistics{
		CadastralMunicipalityCode: cadastralCode,
		BuildingNumber:            buildingNumber,
		SaleHistory:               []BuildingYearPoint{},
		RentalHistory:             []BuildingYearPoint{},
	}
	err := r.pool.QueryRow(ctx, `
		WITH parts AS (
		  SELECT construction_year,building_part_number,location
		  FROM building_parts
		  WHERE cadastral_municipality_code=$1 AND building_number=$2
		  UNION ALL
		  SELECT construction_year,building_part_number,location
		  FROM rental_building_parts
		  WHERE cadastral_municipality_code=$1 AND building_number=$2
		)
		SELECT min(construction_year),
		  count(DISTINCT building_part_number) FILTER(WHERE building_part_number IS NOT NULL),
		  ST_X(ST_Transform(ST_Centroid(ST_Collect(location)),4326)),
		  ST_Y(ST_Transform(ST_Centroid(ST_Collect(location)),4326))
		FROM parts HAVING count(*)>0`, cadastralCode, buildingNumber).Scan(
		&result.ConstructionYear, &result.KnownBuildingPartCount,
		&result.Longitude, &result.Latitude)
	if errors.Is(err, pgx.ErrNoRows) {
		return BuildingStatistics{}, ErrBuildingNotFound
	}
	if err != nil {
		return BuildingStatistics{}, fmt.Errorf("load building identity: %w", err)
	}
	if err := r.loadBuildingSummary(ctx, "analytics.sale_facts", "price",
		"price_per_sqm", cadastralCode, buildingNumber, &result.Sales); err != nil {
		return BuildingStatistics{}, err
	}
	if err := r.loadBuildingSummary(ctx, "analytics.rental_facts",
		"monthly_rent_ex_vat", "monthly_rent_per_sqm", cadastralCode,
		buildingNumber, &result.Rentals); err != nil {
		return BuildingStatistics{}, err
	}
	result.SaleHistory, err = r.loadBuildingHistory(ctx, "analytics.sale_facts",
		"price_per_sqm", cadastralCode, buildingNumber)
	if err != nil {
		return BuildingStatistics{}, err
	}
	result.RentalHistory, err = r.loadBuildingHistory(ctx, "analytics.rental_facts",
		"monthly_rent_per_sqm", cadastralCode, buildingNumber)
	return result, err
}

func (r *StatisticsRepository) loadBuildingSummary(
	ctx context.Context, table, amount, perSQM string, code int32,
	number string, result *BuildingMarketSummary,
) error {
	query := fmt.Sprintf(`
		SELECT count(*)::bigint,
		  count(*) FILTER(WHERE representative_eligible)::bigint,
		  percentile_cont(.5) WITHIN GROUP(ORDER BY %s)
		    FILTER(WHERE representative_eligible)::float8,
		  percentile_cont(.5) WITHIN GROUP(ORDER BY %s)
		    FILTER(WHERE representative_eligible)::float8,
		  min(contract_date)::text,max(contract_date)::text
		FROM %s WHERE cadastral_municipality_code=$1 AND building_number=$2`,
		amount, perSQM, table)
	if err := r.pool.QueryRow(ctx, query, code, number).Scan(
		&result.TransactionCount, &result.RepresentativeCount,
		&result.MedianAmount, &result.MedianAmountPerSQM,
		&result.FirstContractDate, &result.LastContractDate); err != nil {
		return fmt.Errorf("load building market summary: %w", err)
	}
	return nil
}

func (r *StatisticsRepository) loadBuildingHistory(
	ctx context.Context, table, perSQM string, code int32, number string,
) ([]BuildingYearPoint, error) {
	query := fmt.Sprintf(`
		SELECT extract(year FROM contract_date)::integer,count(*)::bigint,
		  count(*) FILTER(WHERE representative_eligible)::bigint,
		  percentile_cont(.5) WITHIN GROUP(ORDER BY %s)
		    FILTER(WHERE representative_eligible)::float8
		FROM %s
		WHERE cadastral_municipality_code=$1 AND building_number=$2
		  AND contract_date IS NOT NULL
		GROUP BY 1 ORDER BY 1`, perSQM, table)
	rows, err := r.pool.Query(ctx, query, code, number)
	if err != nil {
		return nil, fmt.Errorf("load building history: %w", err)
	}
	defer rows.Close()
	items := make([]BuildingYearPoint, 0)
	for rows.Next() {
		var item BuildingYearPoint
		if err := rows.Scan(&item.Year, &item.TransactionCount,
			&item.RepresentativeCount, &item.MedianAmountPerSQM); err != nil {
			return nil, fmt.Errorf("scan building history: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
