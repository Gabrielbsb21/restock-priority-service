package application

import (
	"context"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
)

type PriorityService struct {
	repo   PartRepository
	engine *domain.PriorityEngine
}

func NewPriorityService(repo PartRepository, engine *domain.PriorityEngine) *PriorityService {
	return &PriorityService{
		repo:   repo,
		engine: engine,
	}
}

func (s *PriorityService) GetRestockPriorities(ctx context.Context) ([]domain.PriorityItem, error) {
	parts, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	priorities := s.engine.CalculateAndRank(parts)
	if priorities == nil {
		priorities = []domain.PriorityItem{}
	}

	return priorities, nil
}
