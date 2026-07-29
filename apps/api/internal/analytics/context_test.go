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
