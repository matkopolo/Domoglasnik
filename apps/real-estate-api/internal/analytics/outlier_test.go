package analytics

import "testing"

func TestIsStatisticalOutlier(t *testing.T) {
	tests := []struct {
		name                    string
		value, q1, q3, med, mad float64
		want                    bool
	}{
		{"typical apartment", 2000, 1800, 2200, 2000, 150, false},
		{"extreme apartment", 16000, 1800, 2200, 2000, 150, true},
		{"lower extreme", 200, 1800, 2200, 2000, 150, true},
		{"zero dispersion exact match", 2000, 2000, 2000, 2000, 0, false},
		{"MAD catches narrow center", 4000, 1000, 4000, 2000, 100, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IsStatisticalOutlier(test.value, test.q1, test.q3, test.med, test.mad)
			if got != test.want {
				t.Fatalf("IsStatisticalOutlier()=%v want=%v", got, test.want)
			}
		})
	}
}
