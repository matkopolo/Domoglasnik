package economics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

type InflationFactor struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Factor float64 `json:"factor"`
	Months int     `json:"months"`
	Source string  `json:"source"`
}

// Chain monthly CPI ratios, never overlapping year-on-year rates. Rounded published
// monthly ratios give an approximate adjustment, not a daily price index.
func (c *Client) InflationFactor(ctx context.Context, from, to time.Time) (InflationFactor, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ratios == nil || time.Since(c.ratiosAt) >= 6*time.Hour {
		body := `{"query":[{"code":"MESEC","selection":{"filter":"all","values":["*"]}},{"code":"INDEKS","selection":{"filter":"item","values":["1"]}}],"response":{"format":"json-stat2"}}`
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBufferString(body))
		if err != nil {
			return InflationFactor{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return InflationFactor{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return InflationFactor{}, fmt.Errorf("SURS monthly CPI status %d", resp.StatusCode)
		}
		var payload pxResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return InflationFactor{}, err
		}
		if payload.Dataset != nil {
			payload = *payload.Dataset
		}
		dim, ok := payload.Dimension["MESEC"]
		if !ok {
			return InflationFactor{}, fmt.Errorf("SURS monthly CPI missing dimension")
		}
		// The request selects a single index. Reject a differently shaped response rather
		// than accidentally treating another index as a monthly ratio.
		if len(payload.ID) != 2 || payload.ID[0] != "MESEC" || payload.ID[1] != "INDEKS" || len(payload.Size) != 2 || payload.Size[1] != 1 {
			return InflationFactor{}, fmt.Errorf("SURS monthly CPI unexpected dimensions")
		}
		ratios := map[string]float64{}
		for month, index := range dim.Category.Index {
			if index < 0 || index >= len(payload.Value) {
				continue
			}
			v, ok := payload.Value[index].(float64)
			if ok && v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) {
				ratios[strings.Replace(month, "M", "-", 1)+"-01"] = v / 100
			}
		}
		c.ratios = ratios
		c.ratiosAt = time.Now()
	}
	return chainMonthlyRatios(c.ratios, from, to)
}

func chainMonthlyRatios(ratios map[string]float64, from, to time.Time) (InflationFactor, error) {
	result := InflationFactor{From: monthStart(from).Format("2006-01-02"), To: monthStart(to).Format("2006-01-02"), Factor: 1, Source: "SURS H281S · INDEKS=1"}
	if from.After(to) {
		return result, fmt.Errorf("reversed CPI interval")
	}
	// Require the endpoint months to have been published, even for same-month sales.
	if _, ok := ratios[result.From]; !ok {
		return result, fmt.Errorf("missing CPI month %s", result.From)
	}
	if _, ok := ratios[result.To]; !ok {
		return result, fmt.Errorf("missing CPI month %s", result.To)
	}
	for month := monthStart(from).AddDate(0, 1, 0); !month.After(monthStart(to)); month = month.AddDate(0, 1, 0) {
		v, ok := ratios[month.Format("2006-01-02")]
		if !ok || v <= 0 {
			return result, fmt.Errorf("incomplete CPI series")
		}
		result.Factor *= v
		result.Months++
	}
	return result, nil
}
