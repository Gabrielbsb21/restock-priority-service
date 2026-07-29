package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/adapter/memory"
	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errRepository is a deliberate failure, standing in for an unreachable database.
var errRepository = errors.New("connection refused")

// failingRepository fails every call. It is a small explicit stub rather than a
// generated mock: the tests only need to prove the error reaches the caller.
type failingRepository struct{}

func (failingRepository) Create(context.Context, *domain.Part) error { return errRepository }
func (failingRepository) GetByID(context.Context, uuid.UUID) (*domain.Part, error) {
	return nil, errRepository
}
func (failingRepository) List(context.Context, application.ListFilter) ([]*domain.Part, int, error) {
	return nil, 0, errRepository
}
func (failingRepository) Update(context.Context, *domain.Part) error { return errRepository }
func (failingRepository) Delete(context.Context, uuid.UUID) error    { return errRepository }
func (failingRepository) ListAll(context.Context) ([]*domain.Part, error) {
	return nil, errRepository
}

// updateFailingRepository finds parts but cannot write them, so UpdatePart reaches its
// second repository call instead of returning at the lookup.
type updateFailingRepository struct {
	*memory.PartRepository
}

func (updateFailingRepository) Update(context.Context, *domain.Part) error { return errRepository }

func newPart(name, category string) *domain.Part {
	return &domain.Part{
		Name:              name,
		Category:          category,
		CurrentStock:      10,
		MinimumStock:      15,
		AverageDailySales: decimal.RequireFromString("2"),
		LeadTimeDays:      5,
		UnitCost:          decimal.RequireFromString("12.50"),
		CriticalityLevel:  3,
	}
}

// TestPartService_CreatePart covers FR-001 and the "without persisting" half of
// AC-004.
func TestPartService_CreatePart(t *testing.T) {
	t.Parallel()

	t.Run("generates an identifier and persists", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID)

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Air Filter A", stored.Name)
	})

	t.Run("normalizes name and category before persisting", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("  Oil Filter X  ", " engine "))
		require.NoError(t, err)

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Oil Filter X", stored.Name)
		assert.Equal(t, "engine", stored.Category)
	})

	t.Run("AC-004 an invalid part is not persisted", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		invalid := newPart("Bad Part", "engine")
		invalid.CriticalityLevel = 9

		_, err := service.CreatePart(context.Background(), invalid)

		var fieldErrs domain.FieldErrors
		require.ErrorAs(t, err, &fieldErrs)
		assert.Contains(t, fieldErrs, "criticalityLevel")

		_, total, listErr := service.ListParts(context.Background(), application.ListFilter{Limit: 10})
		require.NoError(t, listErr)
		assert.Zero(t, total, "nothing should have been written")
	})
}

func TestPartService_GetPartByID(t *testing.T) {
	t.Parallel()

	repo := memory.NewPartRepository()
	service := application.NewPartService(repo)

	created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
	require.NoError(t, err)

	t.Run("returns a stored part", func(t *testing.T) {
		found, err := service.GetPartByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "Air Filter A", found.Name)
	})

	t.Run("AC-005 reports a missing part", func(t *testing.T) {
		_, err := service.GetPartByID(context.Background(), uuid.New())
		assert.ErrorIs(t, err, application.ErrPartNotFound)
	})
}

// TestPartService_UpdatePart covers FR-005 and AC-002. The zero-value case is the
// one a struct-based SQL update silently drops.
func TestPartService_UpdatePart(t *testing.T) {
	t.Parallel()

	t.Run("replaces every mutable field", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
		require.NoError(t, err)

		replacement := newPart("Air Filter B", "engine")
		replacement.CurrentStock = 99
		replacement.MinimumStock = 42
		replacement.LeadTimeDays = 7
		replacement.CriticalityLevel = 5

		updated, err := service.UpdatePart(context.Background(), created.ID, replacement)
		require.NoError(t, err)
		assert.Equal(t, created.ID, updated.ID, "the path identifier must be preserved")

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Air Filter B", stored.Name)
		assert.Equal(t, "engine", stored.Category)
		assert.Equal(t, int64(99), stored.CurrentStock)
		assert.Equal(t, int64(42), stored.MinimumStock)
		assert.Equal(t, int32(7), stored.LeadTimeDays)
		assert.Equal(t, 5, stored.CriticalityLevel)
	})

	t.Run("replaces numeric fields with zero", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
		require.NoError(t, err)

		zeroed := newPart("Air Filter A", "filter")
		zeroed.CurrentStock = 0
		zeroed.MinimumStock = 0
		zeroed.LeadTimeDays = 0
		zeroed.AverageDailySales = decimal.Zero
		zeroed.UnitCost = decimal.Zero

		_, err = service.UpdatePart(context.Background(), created.ID, zeroed)
		require.NoError(t, err)

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), stored.CurrentStock)
		assert.Equal(t, int64(0), stored.MinimumStock)
		assert.Equal(t, int32(0), stored.LeadTimeDays)
		assert.True(t, stored.AverageDailySales.IsZero())
		assert.True(t, stored.UnitCost.IsZero())
	})

	t.Run("accepts a negative current stock", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
		require.NoError(t, err)

		negative := newPart("Air Filter A", "filter")
		negative.CurrentStock = -25

		_, err = service.UpdatePart(context.Background(), created.ID, negative)
		require.NoError(t, err)

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(-25), stored.CurrentStock)
	})

	t.Run("AC-005 reports a missing part", func(t *testing.T) {
		t.Parallel()

		service := application.NewPartService(memory.NewPartRepository())

		_, err := service.UpdatePart(context.Background(), uuid.New(), newPart("Ghost", "engine"))
		assert.ErrorIs(t, err, application.ErrPartNotFound)
	})

	t.Run("AC-004 an invalid replacement is not applied", func(t *testing.T) {
		t.Parallel()

		repo := memory.NewPartRepository()
		service := application.NewPartService(repo)

		created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
		require.NoError(t, err)

		invalid := newPart("Air Filter A", "filter")
		invalid.MinimumStock = -1

		_, err = service.UpdatePart(context.Background(), created.ID, invalid)

		var fieldErrs domain.FieldErrors
		require.ErrorAs(t, err, &fieldErrs)
		assert.Contains(t, fieldErrs, "minimumStock")

		stored, err := repo.GetByID(context.Background(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(15), stored.MinimumStock, "the stored value must be untouched")
	})
}

func TestPartService_DeletePart(t *testing.T) {
	t.Parallel()

	repo := memory.NewPartRepository()
	service := application.NewPartService(repo)

	created, err := service.CreatePart(context.Background(), newPart("Air Filter A", "filter"))
	require.NoError(t, err)

	require.NoError(t, service.DeletePart(context.Background(), created.ID))

	_, err = service.GetPartByID(context.Background(), created.ID)
	assert.ErrorIs(t, err, application.ErrPartNotFound)

	// Repeating a successful deletion must report the part as missing.
	assert.ErrorIs(t, service.DeletePart(context.Background(), created.ID), application.ErrPartNotFound)
}

// TestPartService_ListParts covers FR-003 and FR-004.
func TestPartService_ListParts(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T) *application.PartService {
		t.Helper()

		service := application.NewPartService(memory.NewPartRepository())
		for _, spec := range []struct{ name, category string }{
			{"Brake Pad", "brakes"},
			{"apple sensor", "electric"},
			{"Oil Filter", "engine"},
			{"Air Filter", "engine"},
		} {
			_, err := service.CreatePart(context.Background(), newPart(spec.name, spec.category))
			require.NoError(t, err)
		}

		return service
	}

	t.Run("returns every part in the stable default order", func(t *testing.T) {
		t.Parallel()

		parts, total, err := seed(t).ListParts(context.Background(), application.ListFilter{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 4, total)

		names := make([]string, 0, len(parts))
		for _, part := range parts {
			names = append(names, part.Name)
		}
		// Case-insensitive: a byte-wise sort would place "apple sensor" last.
		assert.Equal(t, []string{"Air Filter", "apple sensor", "Brake Pad", "Oil Filter"}, names)
	})

	t.Run("AC-003 filters by exact category and reports the filtered total", func(t *testing.T) {
		t.Parallel()

		parts, total, err := seed(t).ListParts(context.Background(), application.ListFilter{Category: "engine", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 2, total, "the total must describe the filtered result")
		assert.Len(t, parts, 2)
	})

	t.Run("an unknown category returns an empty page", func(t *testing.T) {
		t.Parallel()

		parts, total, err := seed(t).ListParts(context.Background(), application.ListFilter{Category: "ENGINE", Limit: 10})
		require.NoError(t, err)
		assert.Zero(t, total, "categories are case-sensitive")
		assert.Empty(t, parts)
	})

	t.Run("applies the requested window", func(t *testing.T) {
		t.Parallel()

		parts, total, err := seed(t).ListParts(context.Background(), application.ListFilter{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		require.Len(t, parts, 2)
		assert.Equal(t, "Brake Pad", parts[0].Name)
		assert.Equal(t, "Oil Filter", parts[1].Name)
	})

	t.Run("an offset past the end returns an empty page and the real total", func(t *testing.T) {
		t.Parallel()

		parts, total, err := seed(t).ListParts(context.Background(), application.ListFilter{Limit: 10, Offset: 99})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Empty(t, parts)
	})

	// The HTTP adapter rejects these values with a 400 before they arrive, so this
	// clamp is the use case's own contract for any other caller.
	t.Run("clamps out-of-range paging for non-HTTP callers", func(t *testing.T) {
		t.Parallel()

		service := seed(t)

		for _, filter := range []application.ListFilter{
			{Limit: 0},
			{Limit: -1},
			{Limit: 10_000},
			{Limit: 10, Offset: -5},
		} {
			parts, total, err := service.ListParts(context.Background(), filter)
			require.NoError(t, err)
			assert.Equal(t, 4, total)
			assert.Len(t, parts, 4, "filter %+v should return every seeded part", filter)
		}
	})
}

// TestPartService_PropagatesRepositoryErrors is what lets the HTTP adapter answer 500
// with a logged cause instead of panicking.
func TestPartService_PropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	service := application.NewPartService(failingRepository{})
	ctx := context.Background()

	_, createErr := service.CreatePart(ctx, newPart("Air Filter A", "filter"))
	assert.ErrorIs(t, createErr, errRepository)

	_, getErr := service.GetPartByID(ctx, uuid.New())
	assert.ErrorIs(t, getErr, errRepository)

	_, _, listErr := service.ListParts(ctx, application.ListFilter{Limit: 10})
	assert.ErrorIs(t, listErr, errRepository)

	_, updateErr := service.UpdatePart(ctx, uuid.New(), newPart("Air Filter A", "filter"))
	assert.ErrorIs(t, updateErr, errRepository)

	assert.ErrorIs(t, service.DeletePart(ctx, uuid.New()), errRepository)
}

// TestPartService_UpdatePart_PropagatesWriteFailure reaches the branch a repository that
// fails every call cannot: the part is found, and only the write fails.
func TestPartService_UpdatePart_PropagatesWriteFailure(t *testing.T) {
	t.Parallel()

	repo := memory.NewPartRepository()

	existing := newPart("Air Filter A", "filter")
	existing.ID = uuid.New()
	require.NoError(t, repo.Create(context.Background(), existing))

	service := application.NewPartService(updateFailingRepository{repo})

	_, err := service.UpdatePart(context.Background(), existing.ID, newPart("Air Filter B", "engine"))
	assert.ErrorIs(t, err, errRepository)
}
