package main

import (
	"context"
	"errors"
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
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	var db *gorm.DB
	if cfg.DatabaseURL != "" {
		gormDB, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
		if err != nil {
			slog.Warn("could not connect to database at startup", "error", err)
		} else {
			db = gormDB
			slog.Info("connected to postgresql successfully")

			// Run auto-migration for Parts table
			if err := db.AutoMigrate(&adapterPG.PartModel{}); err != nil {
				slog.Error("failed to run database auto-migration", "error", err)
			}
		}
	}

	var repo application.PartRepository
	if db != nil {
		repo = adapterPG.NewPartRepository(db)
	}

	partService := application.NewPartService(repo)
	priorityEngine := domain.NewPriorityEngine()
	priorityService := application.NewPriorityService(repo, priorityEngine)

	router := adapterHTTP.NewRouter(cfg.GinMode, partService, priorityService, db)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting HTTP server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down HTTP server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server shutdown complete")
}
