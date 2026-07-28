package application_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

type FakePartRepository struct {
	parts map[uuid.UUID]*domain.Part
}

func NewFakePartRepository() *FakePartRepository {
	return &FakePartRepository{
		parts: make(map[uuid.UUID]*domain.Part),
	}
}

func (r *FakePartRepository) Create(ctx context.Context, part *domain.Part) error {
	r.parts[part.ID] = part
	return nil
}

func (r *FakePartRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Part, error) {
	p, ok := r.parts[id]
	if !ok {
		return nil, application.ErrPartNotFound
	}
	return p, nil
}

func (r *FakePartRepository) List(ctx context.Context, filter application.ListFilter) ([]*domain.Part, int, error) {
	var list []*domain.Part
	for _, p := range r.parts {
		if filter.Category != "" && p.Category != filter.Category {
			continue
		}
		list = append(list, p)
	}

	sort.Slice(list, func(i, j int) bool {
		aFold := strings.ToLower(list[i].Name)
		bFold := strings.ToLower(list[j].Name)
		if aFold != bFold {
			return aFold < bFold
		}
		return list[i].ID.String() < list[j].ID.String()
	})

	total := len(list)
	start := filter.Offset
	if start >= total {
		return []*domain.Part{}, total, nil
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}

func (r *FakePartRepository) Update(ctx context.Context, part *domain.Part) error {
	if _, ok := r.parts[part.ID]; !ok {
		return application.ErrPartNotFound
	}
	r.parts[part.ID] = part
	return nil
}

func (r *FakePartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.parts[id]; !ok {
		return application.ErrPartNotFound
	}
	delete(r.parts, id)
	return nil
}

func (r *FakePartRepository) ListAll(ctx context.Context) ([]*domain.Part, error) {
	var list []*domain.Part
	for _, p := range r.parts {
		list = append(list, p)
	}
	return list, nil
}

func TestPartService_CRUD(t *testing.T) {
	repo := NewFakePartRepository()
	service := application.NewPartService(repo)
	ctx := context.Background()

	// Create Part
	partInput := &domain.Part{
		Name:              "Air Filter A",
		Category:          "Filter",
		CurrentStock:      10,
		MinimumStock:      15,
		AverageDailySales: decimal.NewFromFloat(2),
		LeadTimeDays:      5,
		UnitCost:          decimal.NewFromFloat(12.50),
		CriticalityLevel:  3,
	}

	created, err := service.CreatePart(ctx, partInput)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)

	// Get Part By ID
	found, err := service.GetPartByID(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Air Filter A", found.Name)

	// List Parts
	list, total, err := service.ListParts(ctx, application.ListFilter{Limit: 10, Offset: 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)

	// Delete Part
	err = service.DeletePart(ctx, created.ID)
	assert.NoError(t, err)

	_, err = service.GetPartByID(ctx, created.ID)
	assert.Equal(t, application.ErrPartNotFound, err)
}
