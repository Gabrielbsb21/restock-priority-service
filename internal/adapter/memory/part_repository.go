// Package memory holds an in-process implementation of the persistence ports.
//
// It exists for two reasons: tests get a fast repository without a database, and
// the service demonstrates that the application depends on its own ports rather
// than on PostgreSQL. Swapping databases means adding a package like this one.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
)

var (
	_ application.PartRepository   = (*PartRepository)(nil)
	_ application.ReadinessChecker = (*PartRepository)(nil)
)

// PartRepository stores parts in a map guarded by a mutex.
type PartRepository struct {
	mu    sync.RWMutex
	parts map[uuid.UUID]domain.Part
}

func NewPartRepository() *PartRepository {
	return &PartRepository{parts: make(map[uuid.UUID]domain.Part)}
}

func (r *PartRepository) Create(_ context.Context, part *domain.Part) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.parts[part.ID] = *part
	return nil
}

func (r *PartRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Part, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stored, ok := r.parts[id]
	if !ok {
		return nil, application.ErrPartNotFound
	}

	// Return a copy so callers cannot mutate stored state by holding the pointer,
	// which is how the SQL adapter behaves.
	return &stored, nil
}

// List applies the filter, then the stable default ordering of LOWER(name) ASC,
// id ASC, then the window. It mirrors the SQL adapter: a negative Limit means no
// limit and a zero Limit returns no rows. PartService clamps the filter first.
func (r *PartRepository) List(_ context.Context, filter application.ListFilter) ([]*domain.Part, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matching := make([]domain.Part, 0, len(r.parts))
	for _, part := range r.parts {
		if filter.Category != "" && part.Category != filter.Category {
			continue
		}
		matching = append(matching, part)
	}

	sort.Slice(matching, func(i, j int) bool {
		left, right := strings.ToLower(matching[i].Name), strings.ToLower(matching[j].Name)
		if left != right {
			return left < right
		}
		return matching[i].ID.String() < matching[j].ID.String()
	})

	total := len(matching)

	// Clamp both ends. GORM normalizes a negative offset to zero, so this adapter
	// must too: the two implementations of the port have to agree on edge cases even
	// though PartService currently clamps before either is reached.
	start := filter.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := total
	if filter.Limit >= 0 {
		end = start + filter.Limit
		if end > total {
			end = total
		}
	}

	page := make([]*domain.Part, 0, end-start)
	for i := start; i < end; i++ {
		part := matching[i]
		page = append(page, &part)
	}

	return page, total, nil
}

func (r *PartRepository) Update(_ context.Context, part *domain.Part) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.parts[part.ID]; !ok {
		return application.ErrPartNotFound
	}

	r.parts[part.ID] = *part
	return nil
}

func (r *PartRepository) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.parts[id]; !ok {
		return application.ErrPartNotFound
	}

	delete(r.parts, id)
	return nil
}

// ListAll returns every part for ranking. Order is unspecified: the priority engine
// establishes the total order itself.
func (r *PartRepository) ListAll(_ context.Context) ([]*domain.Part, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*domain.Part, 0, len(r.parts))
	for _, part := range r.parts {
		stored := part
		all = append(all, &stored)
	}

	return all, nil
}

// CheckReadiness always succeeds: an in-process map has no dependency to reach.
func (r *PartRepository) CheckReadiness(_ context.Context) error {
	return nil
}
