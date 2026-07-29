package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PriorityItem is a ranked restock candidate. It carries no serialization tags on
// purpose: the transport layer owns the wire contract and maps this type into its
// own response struct.
type PriorityItem struct {
	PartID         uuid.UUID
	Name           string
	CurrentStock   int64
	ProjectedStock decimal.Decimal
	MinimumStock   int64
	UrgencyScore   decimal.Decimal

	// Internal fields for sorting criteria (BR-006, BR-007)
	criticalityLevel  int
	averageDailySales decimal.Decimal
}

// PriorityEngine contains pure business calculation methods.
type PriorityEngine struct{}

func NewPriorityEngine() *PriorityEngine {
	return &PriorityEngine{}
}

func (pe *PriorityEngine) CalculateAndRank(parts []*Part) []PriorityItem {
	var eligible []PriorityItem

	for _, part := range parts {
		leadTimeDec := decimal.NewFromInt(int64(part.LeadTimeDays))
		expectedConsumption := part.AverageDailySales.Mul(leadTimeDec)

		currentStockDec := decimal.NewFromInt(part.CurrentStock)
		projectedStock := currentStockDec.Sub(expectedConsumption)

		minimumStockDec := decimal.NewFromInt(part.MinimumStock)

		// BR-003: Restock required if and only if projectedStock < minimumStock
		if projectedStock.LessThan(minimumStockDec) {
			// BR-004: urgencyScore = (minimumStock - projectedStock) * criticalityLevel
			criticalityDec := decimal.NewFromInt(int64(part.CriticalityLevel))
			shortage := minimumStockDec.Sub(projectedStock)
			urgencyScore := shortage.Mul(criticalityDec)

			eligible = append(eligible, PriorityItem{
				PartID:            part.ID,
				Name:              part.Name,
				CurrentStock:      part.CurrentStock,
				ProjectedStock:    projectedStock,
				MinimumStock:      part.MinimumStock,
				UrgencyScore:      urgencyScore,
				criticalityLevel:  part.CriticalityLevel,
				averageDailySales: part.AverageDailySales,
			})
		}
	}

	// Sort eligible parts strictly following BR-005 to BR-009
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]

		// BR-005: urgencyScore descending
		if !a.UrgencyScore.Equal(b.UrgencyScore) {
			return a.UrgencyScore.GreaterThan(b.UrgencyScore)
		}

		// BR-006: criticalityLevel descending
		if a.criticalityLevel != b.criticalityLevel {
			return a.criticalityLevel > b.criticalityLevel
		}

		// BR-007: averageDailySales descending
		if !a.averageDailySales.Equal(b.averageDailySales) {
			return a.averageDailySales.GreaterThan(b.averageDailySales)
		}

		// BR-008: name ascending (case-insensitive Unicode fold)
		aFold := strings.ToLower(a.Name)
		bFold := strings.ToLower(b.Name)
		if aFold != bFold {
			return aFold < bFold
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}

		// BR-009: ID ascending for deterministic final tie-breaker
		return a.PartID.String() < b.PartID.String()
	})

	return eligible
}
