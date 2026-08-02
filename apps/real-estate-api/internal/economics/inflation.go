package economics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultSURSURL = "https://pxweb.stat.si/SiStatData/api/v1/en/Data/H281S.px"

type Point struct {
	Period            string  `json:"period"`
	AnnualRatePercent float64 `json:"annualRatePercent"`
}

type Store interface {
	Inflation(context.Context, time.Time, time.Time, string) ([]Point, error)
}

type Client struct {
	url        string
	httpClient *http.Client
	mu         sync.Mutex
	cachedAt   time.Time
	monthly    []Point
}

func NewClient(httpClient *http.Client, url string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	if url == "" {
		url = DefaultSURSURL
	}
	return &Client{url: url, httpClient: httpClient}
}

type pxResponse struct {
	ID        []string `json:"id"`
	Size      []int    `json:"size"`
	Value     []any    `json:"value"`
	Dimension map[string]struct {
		Category struct {
			Index map[string]int `json:"index"`
		} `json:"category"`
	} `json:"dimension"`
	Dataset *pxResponse `json:"dataset"`
}

func (c *Client) Inflation(ctx context.Context, from, to time.Time, interval string) ([]Point, error) {
	monthly, err := c.load(ctx)
	if err != nil {
		return nil, err
	}
	groups := map[string][]float64{}
	for _, point := range monthly {
		period, parseErr := time.Parse("2006-01-02", point.Period)
		if parseErr != nil || period.Before(monthStart(from)) || period.After(monthStart(to)) {
			continue
		}
		key := period.Format("2006-01-02")
		switch interval {
		case "quarter":
			key = fmt.Sprintf("%04d-%02d-01", period.Year(), ((int(period.Month())-1)/3)*3+1)
		case "year":
			key = fmt.Sprintf("%04d-01-01", period.Year())
		}
		groups[key] = append(groups[key], point.AnnualRatePercent)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]Point, 0, len(keys))
	for _, key := range keys {
		var total float64
		for _, value := range groups[key] {
			total += value
		}
		result = append(result, Point{Period: key, AnnualRatePercent: total / float64(len(groups[key]))})
	}
	return result, nil
}

func (c *Client) load(ctx context.Context) ([]Point, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.cachedAt) < 6*time.Hour && c.monthly != nil {
		return append([]Point(nil), c.monthly...), nil
	}
	body := `{"query":[{"code":"MESEC","selection":{"filter":"all","values":["*"]}},{"code":"INDEKS","selection":{"filter":"item","values":["2"]}}],"response":{"format":"json-stat2"}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBufferString(body))
	if err != nil {
		return nil, fmt.Errorf("create SURS inflation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("load SURS inflation: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("load SURS inflation: status %d", resp.StatusCode)
	}
	var payload pxResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode SURS inflation: %w", err)
	}
	if payload.Dataset != nil {
		payload = *payload.Dataset
	}
	monthDimension, ok := payload.Dimension["MESEC"]
	if !ok {
		return nil, fmt.Errorf("decode SURS inflation: missing MESEC dimension")
	}
	points := make([]Point, 0, len(monthDimension.Category.Index))
	for month, index := range monthDimension.Category.Index {
		if index >= len(payload.Value) || payload.Value[index] == nil {
			continue
		}
		value, ok := payload.Value[index].(float64)
		if !ok || !strings.Contains(month, "M") {
			continue
		}
		points = append(points, Point{
			Period:            strings.Replace(month, "M", "-", 1) + "-01",
			AnnualRatePercent: value - 100,
		})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Period < points[j].Period })
	c.monthly, c.cachedAt = points, time.Now()
	return append([]Point(nil), points...), nil
}

func monthStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}
