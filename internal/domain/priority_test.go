package domain_test

import (
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestPriorityEngine_AC006_ChallengeExample(t *testing.T) {
	// AC-006: Stock 15, Sales 4, Lead Time 5, Min Stock 20, Criticality 3
	// expectedConsumption = 4 * 5 = 20
	// projectedStock = 15 - 20 = -5
	// urgencyScore = (20 - (-5)) * 3 = 75
	id := uuid.New()
	part := &domain.Part{
		ID:                id,
		Name:              "Oil Filter X",
		Category:          "engine",
		CurrentStock:      15,
		MinimumStock:      20,
		AverageDailySales: decimal.NewFromInt(4),
		LeadTimeDays:      5,
		UnitCost:          decimal.NewFromFloat(18.50),
		CriticalityLevel:  3,
	}

	engine := domain.NewPriorityEngine()
	results := engine.CalculateAndRank([]*domain.Part{part})

	assert.Len(t, results, 1)
	assert.Equal(t, id, results[0].PartID)
	assert.Equal(t, "Oil Filter X", results[0].Name)
	assert.Equal(t, int64(15), results[0].CurrentStock)
	assert.True(t, decimal.NewFromInt(-5).Equal(results[0].ProjectedStock), "projectedStock should be -5")
	assert.Equal(t, int64(20), results[0].MinimumStock)
	assert.True(t, decimal.NewFromInt(75).Equal(results[0].UrgencyScore), "urgencyScore should be 75")
}

func TestPriorityEngine_AC007_ProjectedStockEqualsMinStock(t *testing.T) {
	// Projected stock = 20 - (2 * 5) = 10, Min stock = 10 -> Excluded
	part := &domain.Part{
		ID:                uuid.New(),
		Name:              "Sufficient Part",
		Category:          "brakes",
		CurrentStock:      20,
		MinimumStock:      10,
		AverageDailySales: decimal.NewFromInt(2),
		LeadTimeDays:      5,
		CriticalityLevel:  2,
	}

	engine := domain.NewPriorityEngine()
	results := engine.CalculateAndRank([]*domain.Part{part})
	assert.Empty(t, results, "Part where projected stock equals minimum stock must be excluded")
}

func TestPriorityEngine_AC010_TieBreakerOrdering(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	// Two parts with identical urgency score (10 * 1 = 10), identical criticality (1), identical sales (1)
	partA := &domain.Part{
		ID:                id2,
		Name:              "Brake Pad B",
		CurrentStock:      0,
		MinimumStock:      10,
		AverageDailySales: decimal.NewFromInt(1),
		LeadTimeDays:      0,
		CriticalityLevel:  1,
	}
	partB := &domain.Part{
		ID:                id1,
		Name:              "Brake Pad A",
		CurrentStock:      0,
		MinimumStock:      10,
		AverageDailySales: decimal.NewFromInt(1),
		LeadTimeDays:      0,
		CriticalityLevel:  1,
	}

	engine := domain.NewPriorityEngine()
	results := engine.CalculateAndRank([]*domain.Part{partA, partB})

	assert.Len(t, results, 2)
	assert.Equal(t, "Brake Pad A", results[0].Name, "Name ascending should come first")
	assert.Equal(t, "Brake Pad B", results[1].Name)
}
