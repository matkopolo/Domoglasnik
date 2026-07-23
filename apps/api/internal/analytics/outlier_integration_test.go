package analytics

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOutlierRuleIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	tests := []struct {
		value, q1, q3, median, mad float64
		want                       bool
	}{
		{2000, 1800, 2200, 2000, 150, false},
		{16000, 1800, 2200, 2000, 150, true},
	}
	for _, test := range tests {
		var got bool
		err := pool.QueryRow(ctx, `
			SELECT analytics.is_statistical_outlier($1,$2,$3,$4,$5)`,
			test.value, test.q1, test.q3, test.median, test.mad,
		).Scan(&got)
		if err != nil || got != test.want {
			t.Fatalf("SQL outlier=%v error=%v want=%v", got, err, test.want)
		}
	}
}
