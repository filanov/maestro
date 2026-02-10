package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/filanov/maestro/internal/api/grpc"
	"github.com/filanov/maestro/internal/api/rest"
	"github.com/filanov/maestro/internal/cleaner"
	"github.com/filanov/maestro/internal/config"
	"github.com/filanov/maestro/internal/db/postgres"
	"github.com/filanov/maestro/internal/engine"
	"github.com/filanov/maestro/internal/monitor"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadServiceConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	setupLogging(cfg.LogLevel, cfg.LogFormat)

	slog.Info("starting maestro service")

	if err := runMigrations(cfg.DBURL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	db, err := postgres.New(
		cfg.DBURL,
		cfg.DBMaxOpenConns,
		cfg.DBMaxIdleConns,
		cfg.DBConnMaxLifetime,
		cfg.DBConnMaxIdleTime,
	)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	slog.Info("database connected")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthMonitor := monitor.NewHealthMonitor(
		db,
		cfg.HealthCheckInterval,
		cfg.HealthMissedThreshold,
	)
	go healthMonitor.Start(ctx)

	cleanerSvc := cleaner.NewCleaner(
		db,
		cfg.CleanerInterval,
		cfg.RetentionDays,
		cfg.MaxExecutionsPerTask,
	)
	go cleanerSvc.Start(ctx)

	scheduler := engine.NewScheduler(db)

	grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPCHost, cfg.GRPCPort)
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", grpcAddr, err)
	}

	grpcServer, err := grpc.StartGRPCServer(grpcAddr, db)
	if err != nil {
		return fmt.Errorf("create grpc server: %w", err)
	}

	go func() {
		slog.Info("grpc server listening", "address", grpcAddr)
		if err := grpcServer.Serve(listener); err != nil {
			slog.Error("grpc server failed", "error", err)
		}
	}()

	restServer := rest.NewServer(db, scheduler, cfg.RESTHost, cfg.RESTPort)
	go func() {
		if err := restServer.Start(); err != nil {
			slog.Error("rest server failed", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	slog.Info("shutdown signal received, stopping gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := restServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("rest server shutdown failed", "error", err)
	}

	grpcServer.GracefulStop()

	cancel()

	slog.Info("shutdown complete")
	return nil
}

func runMigrations(dbURL string) error {
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		return fmt.Errorf("migration init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}

func setupLogging(level, format string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	slog.SetDefault(slog.New(handler))
}
