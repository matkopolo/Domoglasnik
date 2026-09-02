package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/analytics"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/config"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/database"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/economics"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/geography"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/httpapi"
	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/identity"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConnections)
	if err != nil {
		return err
	}
	defer pool.Close()

	geographyRepository := geography.NewRepository(pool)
	statisticsRepository := analytics.NewStatisticsRepository(pool)
	identityRepository := identity.NewRepository(pool)
	inflationClient := economics.NewClient(nil, "")
	handler := httpapi.New(httpapi.Dependencies{
		Logger:           logger,
		Database:         pool,
		Geographies:      geographyRepository,
		Statistics:       statisticsRepository,
		Rentals:          statisticsRepository,
		Transactions:     statisticsRepository,
		Distributions:    statisticsRepository,
		Buildings:        statisticsRepository,
		Anomalies:        statisticsRepository,
		Identities:       identityRepository,
		Models:           statisticsRepository,
		Releases:         statisticsRepository,
		Maps:             statisticsRepository,
		Economics:        inflationClient,
		AllowedOrigins:   cfg.AllowedOrigins,
		Environment:      cfg.Environment,
		ApplicationName:  cfg.ApplicationName,
		ApplicationBuild: cfg.ApplicationBuild,
	})

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.Address, "environment", cfg.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func newLogger(level string) *slog.Logger {
	logLevel := slog.LevelInfo
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}
