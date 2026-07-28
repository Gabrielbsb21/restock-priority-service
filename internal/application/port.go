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
