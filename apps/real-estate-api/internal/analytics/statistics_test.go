package analytics

import "testing"

func TestIntervalSQL(t *testing.T) {
	tests := map[string][2]string{
		"month":   {"month", "1 month"},
		"quarter": {"quarter", "3 months"},
		"year":    {"year", "1 year"},
	}
	for interval, want := range tests {
		unit, step := intervalSQL(interval)
		if unit != want[0] || step != want[1] {
			t.Fatalf("intervalSQL(%q)=(%q,%q)", interval, unit, step)
		}
	}
}
