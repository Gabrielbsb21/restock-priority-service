package application_test

import (
	"context"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/adapter/memory"
	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPriorityService(t *testing.T) (*application.PriorityService, *application.PartService) {
	t.Helper()

	repo := memory.NewPartRepository()

	return application.NewPriorityService(repo, domain.NewPriorityEngine()), application.NewPartService(repo)
}

// TestPriorityService_ReturnsRankedParts proves the service delegates to the domain
// engine rather than ranking on its own.
func TestPriorityService_ReturnsRankedParts(t *testing.T) {
	t.Parallel()

	priorities, parts := newPriorityService(t)
	ctx := context.Background()

	seed := func(name string, currentStock, minimumStock int64, criticality int) {
		t.Helper()

		_, err := parts.CreatePart(ctx, &domain.Part{
			Name:              name,
			Category:          "engine",
			CurrentStock:      currentStock,
			MinimumStock:      minimumStock,
			AverageDailySales: decimal.Zero,
			LeadTimeDays:      0,
			UnitCost:          decimal.RequireFromString("1.00"),
			CriticalityLevel:  criticality,
		})
		require.NoError(t, err)
	}

	seed("needs a little", 8, 10, 1) // shortage 2, score 2
	seed("healthy", 500, 10, 5)      // not eligible
	seed("needs a lot", -10, 10, 5)  // shortage 20, score 100

	ranked, err := priorities.GetRestockPriorities(ctx)
	require.NoError(t, err)

	require.Len(t, ranked, 2, "parts that do not need restocking must be absent")
	assert.Equal(t, "needs a lot", ranked[0].Name)
	assert.Equal(t, "needs a little", ranked[1].Name)
	assert.True(t, decimal.RequireFromString("100").Equal(ranked[0].UrgencyScore))
	assert.True(t, decimal.RequireFromString("-10").Equal(ranked[0].ProjectedStock))
}

// TestPriorityService_EmptyRankingIsNotNil covers AC-012: the slice must serialize as
// [] rather than null.
func TestPriorityService_EmptyRankingIsNotNil(t *testing.T) {
	t.Parallel()

	t.Run("no parts at all", func(t *testing.T) {
		t.Parallel()

		priorities, _ := newPriorityService(t)

		ranked, err := priorities.GetRestockPriorities(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, ranked)
		assert.Empty(t, ranked)
	})

	t.Run("no part needs restocking", func(t *testing.T) {
		t.Parallel()

		priorities, parts := newPriorityService(t)

		_, err := parts.CreatePart(context.Background(), &domain.Part{
			Name:              "Healthy Part",
			Category:          "engine",
			CurrentStock:      500,
			MinimumStock:      10,
			AverageDailySales: decimal.RequireFromString("1"),
			LeadTimeDays:      2,
			UnitCost:          decimal.RequireFromString("1.00"),
			CriticalityLevel:  5,
		})
		require.NoError(t, err)

		ranked, err := priorities.GetRestockPriorities(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, ranked)
		assert.Empty(t, ranked)
	})
}

// TestPriorityService_IsRepeatable covers AC-015 through the use case.
func TestPriorityService_IsRepeatable(t *testing.T) {
	t.Parallel()

	priorities, parts := newPriorityService(t)
	ctx := context.Background()

	for _, name := range []string{"Alpha", "alpha", "Beta"} {
		_, err := parts.CreatePart(ctx, &domain.Part{
			Name:              name,
			Category:          "engine",
			CurrentStock:      5,
			MinimumStock:      10,
			AverageDailySales: decimal.RequireFromString("2"),
			LeadTimeDays:      0,
			UnitCost:          decimal.RequireFromString("1.00"),
			CriticalityLevel:  3,
		})
		require.NoError(t, err)
	}

	// The repository iterates a map, so its order varies between calls. The ranking
	// must not.
	first, err := priorities.GetRestockPriorities(ctx)
	require.NoError(t, err)
	require.Len(t, first, 3)

	for range 10 {
		again, err := priorities.GetRestockPriorities(ctx)
		require.NoError(t, err)
		assert.Equal(t, first, again)
	}
}

func TestPriorityService_PropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	service := application.NewPriorityService(failingRepository{}, domain.NewPriorityEngine())

	_, err := service.GetRestockPriorities(context.Background())
	assert.ErrorIs(t, err, errRepository)
}
