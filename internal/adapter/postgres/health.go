package postgres

import (
	"context"
	"fmt"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"gorm.io/gorm"
)

var _ application.ReadinessChecker = (*ReadinessChecker)(nil)

// ReadinessChecker answers the readiness probe by pinging PostgreSQL.
type ReadinessChecker struct {
	db *gorm.DB
}

func NewReadinessChecker(db *gorm.DB) *ReadinessChecker {
	return &ReadinessChecker{db: db}
}

// CheckReadiness pings the database using the caller's context, so the caller's
// deadline bounds how long the probe can take.
func (r *ReadinessChecker) CheckReadiness(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("resolve database handle: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}
