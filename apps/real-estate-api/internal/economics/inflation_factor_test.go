package economics

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMonthlyInflationFactor(t *testing.T) {
	from := time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	ratios := map[string]float64{"2024-01-01": 1.2, "2024-02-01": 1.02, "2024-03-01": 0.99}
	r, err := chainMonthlyRatios(ratios, from, to)
	if err != nil || math.Abs(r.Factor-1.0098) > 1e-10 || r.Months != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	same, err := chainMonthlyRatios(ratios, from, from)
	if err != nil || same.Factor != 1 {
		t.Fatalf("same month %+v %v", same, err)
	}
	delete(ratios, "2024-02-01")
	if _, err := chainMonthlyRatios(ratios, from, to); err == nil {
		t.Fatal("missing month accepted")
	}
	if _, err := chainMonthlyRatios(ratios, to, from); err == nil {
		t.Fatal("reversed dates accepted")
	}
}

func TestMonthlyCPIRequestAndCache(t *testing.T) {
	calls := 0
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"values":["1"]`) {
			t.Fatal("must select monthly, not annual CPI")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":["MESEC","INDEKS"],"size":[2,1],"dimension":{"MESEC":{"category":{"index":{"2024M01":0,"2024M02":1}}}},"value":[100,102]}`))}, nil
	})}, "https://example.test")
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	for range 2 {
		r, err := client.InflationFactor(context.Background(), from, to)
		if err != nil || r.Factor != 1.02 {
			t.Fatalf("%+v %v", r, err)
		}
	}
	if calls != 1 {
		t.Fatalf("requests=%d", calls)
	}
}
