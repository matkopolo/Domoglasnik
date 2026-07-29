package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/matkopolo/NepremicninskiAgent/apps/api/internal/analytics"
)

func (s *server) searchBuildings(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 3 || len(query) > 120 {
		writeError(w, r, http.StatusBadRequest, "invalid_search",
			"q must contain between 3 and 120 characters")
		return
	}
	limit, ok := boundedInteger(w, r, "limit", 10, 1, 20)
	if !ok {
		return
	}
	items, err := s.buildings.SearchBuildings(r.Context(), query, limit)
	if err != nil {
		s.logger.Error("search buildings", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not search buildings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (s *server) buildingStatistics(w http.ResponseWriter, r *http.Request) {
	code64, err := strconv.ParseInt(r.PathValue("cadastralCode"), 10, 32)
	if err != nil || code64 <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_cadastral_code",
			"cadastral code must be a positive integer")
		return
	}
	number := r.PathValue("buildingNumber")
	if number == "" || len(number) > 100 {
		writeError(w, r, http.StatusBadRequest, "invalid_building_number",
			"building number is required")
		return
	}
	filter, ok := parseBuildingFilter(w, r)
	if !ok {
		return
	}
	result, err := s.buildings.BuildingStatistics(
		r.Context(), int32(code64), number, filter)
	if errors.Is(err, analytics.ErrBuildingNotFound) {
		writeError(w, r, http.StatusNotFound, "building_not_found",
			"building was not found")
		return
	}
	if err != nil {
		s.logger.Error("building statistics", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not load building statistics")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseBuildingFilter(
	w http.ResponseWriter, r *http.Request,
) (analytics.BuildingFilter, bool) {
	var filter analytics.BuildingFilter
	var ok bool
	if filter.FromDate, ok = optionalDate(w, r, "from"); !ok {
		return filter, false
	}
	if filter.ToDate, ok = optionalDate(w, r, "to"); !ok {
		return filter, false
	}
	if filter.FromDate != nil && filter.ToDate != nil &&
		filter.FromDate.After(*filter.ToDate) {
		writeError(w, r, http.StatusBadRequest, "invalid_date_range",
			"from must not be after to")
		return filter, false
	}
	filter.PropertyGroup = r.URL.Query().Get("propertyGroup")
	if filter.PropertyGroup != "" && !validPropertyGroup(filter.PropertyGroup) {
		writeError(w, r, http.StatusBadRequest, "invalid_property_group",
			"unsupported propertyGroup")
		return filter, false
	}
	return filter, true
}
