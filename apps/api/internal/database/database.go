package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string, maxConnections int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.MaxConns = maxConnections
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	var schemaReady bool
	if err := pool.QueryRow(pingCtx, `
		SELECT to_regclass('analytics.geographic_units') IS NOT NULL
		   AND to_regclass('analytics.sale_facts') IS NOT NULL
		   AND to_regclass('analytics.sale_outlier_evaluations') IS NOT NULL
		   AND to_regclass('analytics.rental_facts') IS NOT NULL
		   AND to_regclass('analytics.anomaly_signals') IS NOT NULL
		   AND to_regclass('analytics.valuation_predictions') IS NOT NULL
		   AND to_regclass('analytics.market_indices') IS NOT NULL
		   AND to_regclass('public.app_users') IS NOT NULL
		   AND EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis')`,
	).Scan(&schemaReady); err != nil {
		pool.Close()
		return nil, fmt.Errorf("verify database schema: %w", err)
	}
	if !schemaReady {
		pool.Close()
		return nil, fmt.Errorf("verify database schema: required API migration or PostGIS extension is missing")
	}
	return pool, nil
}
