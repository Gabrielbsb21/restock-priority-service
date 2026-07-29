// Command api serves the restock priority HTTP API.
//
// This file is the composition root: it is the only place that knows the complete
// object graph, and the only place that chooses PostgreSQL as the adapter behind the
// application's ports.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	adapterHTTP "github.com/Gabrielbsb21/restock-priority-service/internal/adapter/http"
	adapterPG "github.com/Gabrielbsb21/restock-priority-service/internal/adapter/postgres"
	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/Gabrielbsb21/restock-priority-service/internal/platform/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	readTimeout       = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 5 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("service stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// gorm.Open pings the database, so an unreachable server fails here. Serving
	// anyway would answer every CRUD and ranking request with a panic, so startup
	// fails instead of degrading silently.
	//
	// GORM's own logger is discarded: it writes coloured, unstructured lines, and
	// every failure is already logged once by the boundary that has the request
	// context.
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		return fmt.Errorf("connect to postgresql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("resolve database handle: %w", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Warn("could not close database", "error", closeErr)
		}
	}()

	slog.Info("connected to postgresql")

	// Schema changes are applied by cmd/migrate before this process starts, never
	// from here and never during request handling.

	repo := adapterPG.NewPartRepository(db)
	readiness := adapterPG.NewReadinessChecker(db)

	partService := application.NewPartService(repo)
	priorityService := application.NewPriorityService(repo, domain.NewPriorityEngine())

	router := adapterHTTP.NewRouter(cfg.GinMode, partService, priorityService, readiness)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "port", cfg.Port)
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			serverErr <- listenErr
			return
		}
		serverErr <- nil
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case listenErr := <-serverErr:
		if listenErr != nil {
			return fmt.Errorf("http server: %w", listenErr)
		}
		return nil
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	slog.Info("shutdown complete")

	return nil
}
