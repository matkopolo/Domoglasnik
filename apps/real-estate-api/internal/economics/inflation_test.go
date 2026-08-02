package economics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestInflationParsesAndAggregatesOfficialShape(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"id":["MESEC","INDEKS"],"size":[3,1],"dimension":{"MESEC":{"category":{"index":{"2024M01":0,"2024M02":1,"2024M03":2}}}},"value":[103,104,105]}`,
		))}, nil
	})}
	client := NewClient(httpClient, "https://example.test/inflation")
	from, _ := time.Parse("2006-01-02", "2024-01-01")
	to, _ := time.Parse("2006-01-02", "2024-12-31")
	points, err := client.Inflation(context.Background(), from, to, "quarter")
	if err != nil || len(points) != 1 || points[0].AnnualRatePercent != 4 {
		t.Fatalf("Inflation()=%+v error=%v", points, err)
	}
}
