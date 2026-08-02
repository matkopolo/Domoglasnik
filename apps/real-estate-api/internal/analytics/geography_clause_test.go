package analytics

import "testing"

func TestGeographyFactColumn(t *testing.T) {
	tests := []struct {
		level  string
		column string
		ok     bool
	}{
		{"country", "", true},
		{"statistical_region", "statistical_region_id", true},
		{"municipality", "municipality_id", true},
		{"city_district", "city_district_id", true},
		{"local_community", "local_community_id", true},
		{"settlement", "settlement_id", true},
		{"unknown", "", false},
	}
	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			column, ok := geographyFactColumn(test.level)
			if column != test.column || ok != test.ok {
				t.Fatalf("geographyFactColumn(%q)=(%q,%t), want (%q,%t)",
					test.level, column, ok, test.column, test.ok)
			}
		})
	}
}
