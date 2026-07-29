package application

import (
	"context"
	"errors"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
)

var ErrPartNotFound = errors.New("part_not_found")

type ListFilter struct {
	Category string
	Limit    int
	Offset   int
}

type PartRepository interface {
	Create(ctx context.Context, part *domain.Part) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Part, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.Part, int, error)
	Update(ctx context.Context, part *domain.Part) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListAll(ctx context.Context) ([]*domain.Part, error)
}

// ReadinessChecker reports whether the persistence dependency can serve traffic.
//
// It is a port of its own rather than a PartRepository method so that readiness
// stays separate from the six data-access capabilities, and so that no transport
// code needs to know which database sits behind the repository. The caller owns
// the deadline: implementations must honour the context they are given.
type ReadinessChecker interface {
	CheckReadiness(ctx context.Context) error
}
