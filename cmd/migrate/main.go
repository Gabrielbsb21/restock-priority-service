// Command migrate applies the versioned SQL migrations to PostgreSQL.
//
// It is a separate binary on purpose: migrations must complete before the API
// starts accepting traffic, and they must never run inside request handling or as
// a side effect of application startup.
//
// Usage:
//
//	migrate [up|down|status|version]
//
// The default command is "up".
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Gabrielbsb21/restock-priority-service/internal/platform/config"
	"github.com/Gabrielbsb21/restock-priority-service/migrations"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
)

// migrationsDir is the root of the embedded filesystem, which holds the SQL files
// directly rather than in a subdirectory.
const migrationsDir = "."

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	flag.Parse()
	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	if err := run(command); err != nil {
		slog.Error("migration command failed", "command", command, "error", err)
		os.Exit(1)
	}
}

func run(command string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("could not close database", "error", closeErr)
		}
	}()

	goose.SetLogger(gooseLogger{})
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	switch command {
	case "up":
		return goose.Up(db, migrationsDir)
	case "down":
		return goose.Down(db, migrationsDir)
	case "status":
		return goose.Status(db, migrationsDir)
	case "version":
		return goose.Version(db, migrationsDir)
	default:
		return fmt.Errorf("unknown command %q: expected up, down, status or version", command)
	}
}

// gooseLogger adapts goose's logger so the migrate step emits the same structured
// output as the API service instead of plain lines.
type gooseLogger struct{}

func (gooseLogger) Printf(format string, v ...any) {
	slog.Info(strings.TrimSpace(fmt.Sprintf(format, v...)))
}

func (gooseLogger) Fatalf(format string, v ...any) {
	slog.Error(strings.TrimSpace(fmt.Sprintf(format, v...)))
	os.Exit(1)
}
