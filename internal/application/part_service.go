package application

import (
	"context"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
)

type PartService struct {
	repo PartRepository
}

func NewPartService(repo PartRepository) *PartService {
	return &PartService{repo: repo}
}

func (s *PartService) CreatePart(ctx context.Context, part *domain.Part) (*domain.Part, error) {
	part.ID = uuid.New()

	if err := part.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, part); err != nil {
		return nil, err
	}

	return part, nil
}

func (s *PartService) GetPartByID(ctx context.Context, id uuid.UUID) (*domain.Part, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PartService) ListParts(ctx context.Context, filter ListFilter) ([]*domain.Part, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return s.repo.List(ctx, filter)
}

func (s *PartService) UpdatePart(ctx context.Context, id uuid.UUID, updated *domain.Part) (*domain.Part, error) {
	// Verify part exists
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updated.ID = existing.ID
	if err := updated.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *PartService) DeletePart(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
