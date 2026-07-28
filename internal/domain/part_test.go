package domain_test

import (
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestPart_Validate_Valid(t *testing.T) {
	part := &domain.Part{
		ID:                uuid.New(),
		Name:              "Spark Plug",
		Category:          "Ignition",
		CurrentStock:      10,
		MinimumStock:      15,
		AverageDailySales: decimal.NewFromFloat(2.5),
		LeadTimeDays:      3,
		UnitCost:          decimal.NewFromFloat(4.99),
		CriticalityLevel:  4,
	}

	err := part.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Spark Plug", part.Name)
	assert.Equal(t, "Ignition", part.Category)
}

func TestPart_Validate_InvalidFields(t *testing.T) {
	part := &domain.Part{
		ID:                uuid.New(),
		Name:              "   ",
		Category:          "",
		MinimumStock:      -1,
		AverageDailySales: decimal.NewFromFloat(-0.5),
		LeadTimeDays:      -2,
		UnitCost:          decimal.NewFromFloat(10.123), // > 2 decimal places
		CriticalityLevel:  6,
	}

	err := part.Validate()
	assert.Error(t, err)

	fe, ok := err.(domain.FieldErrors)
	assert.True(t, ok)
	assert.Contains(t, fe, "name")
	assert.Contains(t, fe, "category")
	assert.Contains(t, fe, "minimumStock")
	assert.Contains(t, fe, "averageDailySales")
	assert.Contains(t, fe, "leadTimeDays")
	assert.Contains(t, fe, "unitCost")
	assert.Contains(t, fe, "criticalityLevel")
}
