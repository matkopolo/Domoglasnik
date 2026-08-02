package analytics

import "testing"

func TestBenchmarkDifference(t *testing.T) {
	value, ok := benchmarkDifference(125, 100)
	if !ok || value != 25 {
		t.Fatalf("benchmarkDifference()=(%v,%v), want (25,true)", value, ok)
	}
	for _, input := range [][2]float64{{0, 100}, {100, 0}, {-1, 100}} {
		if _, ok := benchmarkDifference(input[0], input[1]); ok {
			t.Fatalf("benchmarkDifference(%v,%v) should be unavailable",
				input[0], input[1])
		}
	}
}

func TestBenchmarkReliability(t *testing.T) {
	tests := []struct {
		sample int64
		want   string
	}{{4, "unavailable"}, {5, "very_low"}, {9, "very_low"},
		{10, "low"}, {29, "low"}, {30, "moderate"}}
	for _, test := range tests {
		if got := benchmarkReliability(test.sample); got != test.want {
			t.Errorf("benchmarkReliability(%d)=%q want=%q",
				test.sample, got, test.want)
		}
	}
}

func TestDirectComparableTier(t *testing.T) {
	want := []string{
		"same_building_same_year_area_15",
		"same_building_adjacent_year_area_15",
		"same_building_same_year",
		"same_building_other",
	}
	for index, expected := range want {
		if got := directComparableTier(index + 1); got != expected {
			t.Errorf("directComparableTier(%d)=%q want=%q", index+1, got, expected)
		}
	}
}

func TestNearbyBenchmarkProfiles(t *testing.T) {
	year := 2001
	withAge := nearbyBenchmarkProfiles(&year)
	if len(withAge) != 2 || withAge[0].name != "area_15_age_10" ||
		withAge[1].name != "area_25_age_15" {
		t.Fatalf("nearbyBenchmarkProfiles(with age)=%+v", withAge)
	}
	withoutAge := nearbyBenchmarkProfiles(nil)
	if len(withoutAge) != 2 || withoutAge[0].name != "area_15" ||
		withoutAge[1].name != "area_25" {
		t.Fatalf("nearbyBenchmarkProfiles(without age)=%+v", withoutAge)
	}
}

func TestWeightedQuantile(t *testing.T) {
	values := []weightedValue{{value: 100, weight: 1}, {value: 200, weight: 8}, {value: 300, weight: 1}}
	if got := weightedQuantile(values, .5); got != 200 {
		t.Fatalf("weightedQuantile()=%v want=200", got)
	}
	if got := weightedQuantile(values, .9); got != 200 {
		t.Fatalf("weightedQuantile(.9)=%v want=200", got)
	}
}
