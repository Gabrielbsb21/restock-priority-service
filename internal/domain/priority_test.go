package domain_test

import (
	"math"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixed identifiers keep BR-009 assertions meaningful: idA sorts before idB, which
// sorts before idC.
var (
	idA = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	idB = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	idC = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

// assertDecimalEqual compares by value. Comparing decimal.Decimal structs directly
// would fail for equal numbers stored with different exponents, such as 0 and 0.00.
func assertDecimalEqual(t *testing.T, expected string, actual decimal.Decimal) {
	t.Helper()

	want, err := decimal.NewFromString(expected)
	require.NoError(t, err)
	assert.Truef(t, actual.Equal(want), "expected %s, got %s", want, actual)
}

type partOpts struct {
	id                uuid.UUID
	name              string
	currentStock      int64
	minimumStock      int64
	averageDailySales string
	leadTimeDays      int32
	unitCost          string
	criticalityLevel  int
}

func newPart(opts partOpts) *domain.Part {
	if opts.id == uuid.Nil {
		opts.id = idA
	}
	if opts.name == "" {
		opts.name = "Part"
	}
	if opts.averageDailySales == "" {
		opts.averageDailySales = "0"
	}
	if opts.unitCost == "" {
		opts.unitCost = "1.00"
	}
	if opts.criticalityLevel == 0 {
		opts.criticalityLevel = 1
	}

	return &domain.Part{
		ID:                opts.id,
		Name:              opts.name,
		Category:          "engine",
		CurrentStock:      opts.currentStock,
		MinimumStock:      opts.minimumStock,
		AverageDailySales: decimal.RequireFromString(opts.averageDailySales),
		LeadTimeDays:      opts.leadTimeDays,
		UnitCost:          decimal.RequireFromString(opts.unitCost),
		CriticalityLevel:  opts.criticalityLevel,
	}
}

// TestPriorityEngine_Formulas covers BR-001 through BR-004 and BR-010 through
// BR-012, and the calculation half of AC-006 to AC-009 and AC-011.
func TestPriorityEngine_Formulas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		part           partOpts
		wantEligible   bool
		projectedStock string
		urgencyScore   string
	}{
		{
			// AC-006. The challenge's own example prints 5 and 45 for these inputs,
			// contradicting the formulas it states. The formulas are authoritative.
			name:           "AC-006 challenge example yields negative projected stock",
			part:           partOpts{currentStock: 15, minimumStock: 20, averageDailySales: "4", leadTimeDays: 5, criticalityLevel: 3},
			wantEligible:   true,
			projectedStock: "-5",
			urgencyScore:   "75",
		},
		{
			// The example's second row is internally consistent, unlike its first.
			name:           "AC-006 second example row",
			part:           partOpts{currentStock: 8, minimumStock: 10, averageDailySales: "2", leadTimeDays: 5, criticalityLevel: 3},
			wantEligible:   true,
			projectedStock: "-2",
			urgencyScore:   "36",
		},
		{
			// AC-007. BR-003 is a strict comparison, so equality is not a shortage.
			name:         "AC-007 projected stock equal to minimum is excluded",
			part:         partOpts{currentStock: 30, minimumStock: 10, averageDailySales: "4", leadTimeDays: 5, criticalityLevel: 5},
			wantEligible: false,
		},
		{
			name:           "projected stock one unit below minimum is included",
			part:           partOpts{currentStock: 29, minimumStock: 10, averageDailySales: "4", leadTimeDays: 5, criticalityLevel: 1},
			wantEligible:   true,
			projectedStock: "9",
			urgencyScore:   "1",
		},
		{
			name:         "projected stock above minimum is excluded",
			part:         partOpts{currentStock: 100, minimumStock: 10, averageDailySales: "1", leadTimeDays: 2, criticalityLevel: 5},
			wantEligible: false,
		},
		{
			// AC-008 and BR-010: no clamping anywhere in the chain.
			name:           "AC-008 negative current stock is not clamped",
			part:           partOpts{currentStock: -10, minimumStock: 5, averageDailySales: "2", leadTimeDays: 3, criticalityLevel: 4},
			wantEligible:   true,
			projectedStock: "-16",
			urgencyScore:   "84",
		},
		{
			name:           "negative current stock with zero minimum still ranks",
			part:           partOpts{currentStock: -7, minimumStock: 0, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 1},
			wantEligible:   true,
			projectedStock: "-7",
			urgencyScore:   "7",
		},
		{
			// AC-009 and BR-011.
			name:           "AC-009 zero average daily sales consumes nothing over a long lead time",
			part:           partOpts{currentStock: 5, minimumStock: 10, averageDailySales: "0", leadTimeDays: 30, criticalityLevel: 2},
			wantEligible:   true,
			projectedStock: "5",
			urgencyScore:   "10",
		},
		{
			name:           "AC-009 zero lead time consumes nothing at high sales",
			part:           partOpts{currentStock: 5, minimumStock: 10, averageDailySales: "99.75", leadTimeDays: 0, criticalityLevel: 2},
			wantEligible:   true,
			projectedStock: "5",
			urgencyScore:   "10",
		},
		{
			name:         "AC-009 zero sales with healthy stock is excluded",
			part:         partOpts{currentStock: 50, minimumStock: 10, averageDailySales: "0", leadTimeDays: 90, criticalityLevel: 5},
			wantEligible: false,
		},
		{
			name:           "zero sales and zero lead time",
			part:           partOpts{currentStock: 0, minimumStock: 1, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 3},
			wantEligible:   true,
			projectedStock: "0",
			urgencyScore:   "3",
		},
		{
			// A ten-year lead time: 1.5 * 3650 = 5475.
			name:           "very high lead time",
			part:           partOpts{currentStock: 100, minimumStock: 50, averageDailySales: "1.5", leadTimeDays: 3650, criticalityLevel: 5},
			wantEligible:   true,
			projectedStock: "-5375",
			urgencyScore:   "27125",
		},
		{
			// The largest lead time the domain type allows, at one unit per day.
			name:           "maximum lead time does not overflow",
			part:           partOpts{currentStock: 0, minimumStock: 0, averageDailySales: "1", leadTimeDays: math.MaxInt32, criticalityLevel: 1},
			wantEligible:   true,
			projectedStock: "-2147483647",
			urgencyScore:   "2147483647",
		},
		{
			// 9223372036854775807 * 5 exceeds int64. The result must still be exact.
			name:           "urgency score beyond int64 range stays exact",
			part:           partOpts{currentStock: 0, minimumStock: math.MaxInt64, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 5},
			wantEligible:   true,
			projectedStock: "0",
			urgencyScore:   "46116860184273879035",
		},
		{
			name:         "maximum current stock is never a shortage",
			part:         partOpts{currentStock: math.MaxInt64, minimumStock: 10, averageDailySales: "1", leadTimeDays: 1, criticalityLevel: 5},
			wantEligible: false,
		},
		{
			// AC-011 and BR-012: 2.5 * 3 is exactly 7.5.
			name:           "AC-011 fractional sales use exact decimal arithmetic",
			part:           partOpts{currentStock: 10, minimumStock: 8, averageDailySales: "2.5", leadTimeDays: 3, criticalityLevel: 3},
			wantEligible:   true,
			projectedStock: "2.5",
			urgencyScore:   "16.5",
		},
		{
			// 0.1 * 3 is 0.3 in decimal; in binary floating point it is not.
			name:           "BR-012 fractional sales avoid binary floating point drift",
			part:           partOpts{currentStock: 1, minimumStock: 1, averageDailySales: "0.1", leadTimeDays: 3, criticalityLevel: 1},
			wantEligible:   true,
			projectedStock: "0.7",
			urgencyScore:   "0.3",
		},
	}

	engine := domain.NewPriorityEngine()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := engine.CalculateAndRank([]*domain.Part{newPart(tc.part)})

			if !tc.wantEligible {
				assert.Empty(t, got)
				return
			}

			require.Len(t, got, 1)
			assert.Equal(t, tc.part.currentStock, got[0].CurrentStock)
			assert.Equal(t, tc.part.minimumStock, got[0].MinimumStock)
			assertDecimalEqual(t, tc.projectedStock, got[0].ProjectedStock)
			assertDecimalEqual(t, tc.urgencyScore, got[0].UrgencyScore)
		})
	}
}

// TestPriorityEngine_Ordering covers BR-005 through BR-009 and AC-010, one tie level
// per case so each criterion is exercised in isolation.
func TestPriorityEngine_Ordering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parts     []partOpts
		wantOrder []string
	}{
		{
			// BR-005. Shortage 10 at criticality 3 scores 30; shortage 8 at 1 scores 8.
			name: "BR-005 higher urgency score comes first",
			parts: []partOpts{
				{name: "low urgency", currentStock: 2, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 1},
				{name: "high urgency", currentStock: 0, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"high urgency", "low urgency"},
		},
		{
			name: "BR-005 orders three distinct scores descending",
			parts: []partOpts{
				{name: "middle", currentStock: 0, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 2},
				{name: "highest", currentStock: 0, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 5},
				{name: "lowest", currentStock: 0, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 1},
			},
			wantOrder: []string{"highest", "middle", "lowest"},
		},
		{
			// BR-006. Both score 24: shortage 12 x 2 and shortage 8 x 3.
			name: "BR-006 equal scores fall back to higher criticality",
			parts: []partOpts{
				{name: "criticality two", currentStock: -2, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 2},
				{name: "criticality three", currentStock: 2, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"criticality three", "criticality two"},
		},
		{
			// BR-007. Lead time zero keeps consumption identical while sales differ, so
			// score and criticality tie and only sales can break it. The names are
			// chosen so alphabetical order would give the opposite result.
			name: "BR-007 equal scores and criticality fall back to higher sales",
			parts: []partOpts{
				{name: "alpha low sales", currentStock: 5, minimumStock: 10, averageDailySales: "1", leadTimeDays: 0, criticalityLevel: 3},
				{name: "zulu high sales", currentStock: 5, minimumStock: 10, averageDailySales: "9", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"zulu high sales", "alpha low sales"},
		},
		{
			// BR-007 with fractional sales, which only exact decimals compare reliably.
			name: "BR-007 compares fractional sales exactly",
			parts: []partOpts{
				{name: "lower", currentStock: 5, minimumStock: 10, averageDailySales: "0.1", leadTimeDays: 0, criticalityLevel: 3},
				{name: "higher", currentStock: 5, minimumStock: 10, averageDailySales: "0.10001", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"higher", "lower"},
		},
		{
			// BR-008. A byte-wise comparison would put "Banana" before "apple" because
			// 'B' is 66 and 'a' is 97; the case fold must win.
			name: "BR-008 remaining ties order by case-insensitive name",
			parts: []partOpts{
				{name: "Banana", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
				{name: "apple", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"apple", "Banana"},
		},
		{
			// BR-008 secondary: identical folded names, so the original casing decides.
			name: "BR-008 identical folded names order by the original name",
			parts: []partOpts{
				{id: idB, name: "abc", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
				{id: idA, name: "ABC", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
			},
			wantOrder: []string{"ABC", "abc"},
		},
		{
			// AC-010: the whole chain in one dataset, plus an ineligible part that must
			// not appear at all.
			name: "AC-010 full chain with an ineligible part filtered out",
			parts: []partOpts{
				{id: idB, name: "beta", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
				{name: "healthy", currentStock: 500, minimumStock: 10, averageDailySales: "1", leadTimeDays: 1, criticalityLevel: 5},
				{name: "top score", currentStock: -50, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 5},
				{id: idA, name: "Alpha", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
				{name: "more critical", currentStock: 2, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 4},
			},
			wantOrder: []string{"top score", "more critical", "Alpha", "beta"},
		},
	}

	engine := domain.NewPriorityEngine()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parts := make([]*domain.Part, 0, len(tc.parts))
			for _, opts := range tc.parts {
				parts = append(parts, newPart(opts))
			}

			got := engine.CalculateAndRank(parts)

			names := make([]string, 0, len(got))
			for _, item := range got {
				names = append(names, item.Name)
			}
			assert.Equal(t, tc.wantOrder, names)
		})
	}
}

// TestPriorityEngine_CompleteTieOrdersByIdentifier asserts BR-009 on the identifier
// itself, which name-based ordering assertions cannot observe.
func TestPriorityEngine_CompleteTieOrdersByIdentifier(t *testing.T) {
	t.Parallel()

	tied := partOpts{name: "same", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3}

	third, second, first := tied, tied, tied
	third.id, second.id, first.id = idC, idB, idA

	got := domain.NewPriorityEngine().CalculateAndRank([]*domain.Part{
		newPart(third), newPart(second), newPart(first),
	})

	require.Len(t, got, 3)
	assert.Equal(t, []uuid.UUID{idA, idB, idC}, []uuid.UUID{got[0].PartID, got[1].PartID, got[2].PartID})
}

// TestPriorityEngine_UnitCostIsNotARankingInput covers BR-013 and AC-016.
func TestPriorityEngine_UnitCostIsNotARankingInput(t *testing.T) {
	t.Parallel()

	cheap := partOpts{id: idA, name: "same", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3, unitCost: "0.01"}
	expensive := cheap
	expensive.id, expensive.unitCost = idB, "9999.99"

	got := domain.NewPriorityEngine().CalculateAndRank([]*domain.Part{newPart(expensive), newPart(cheap)})

	require.Len(t, got, 2, "unit cost must not change eligibility")
	assert.Equal(t, []uuid.UUID{idA, idB}, []uuid.UUID{got[0].PartID, got[1].PartID},
		"unit cost must not change ordering; the identifier tie-breaker decides")
	assert.True(t, got[0].UrgencyScore.Equal(got[1].UrgencyScore), "unit cost must not change the score")
}

func TestPriorityEngine_EmptyResults(t *testing.T) {
	t.Parallel()

	engine := domain.NewPriorityEngine()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, engine.CalculateAndRank(nil))
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, engine.CalculateAndRank([]*domain.Part{}))
	})

	// AC-012 at the domain level: eligibility, not emptiness of the input.
	t.Run("no part needs restocking", func(t *testing.T) {
		t.Parallel()
		parts := []*domain.Part{
			newPart(partOpts{name: "a", currentStock: 100, minimumStock: 10, averageDailySales: "1", leadTimeDays: 1, criticalityLevel: 5}),
			newPart(partOpts{name: "b", currentStock: 10, minimumStock: 10, averageDailySales: "0", leadTimeDays: 0, criticalityLevel: 5}),
		}
		assert.Empty(t, engine.CalculateAndRank(parts))
	})
}

// TestPriorityEngine_IsDeterministic covers AC-015. The tie-breaker chain ends in the
// identifier, so the ranking is a total order and the input order cannot affect it.
func TestPriorityEngine_IsDeterministic(t *testing.T) {
	t.Parallel()

	specs := []partOpts{
		{id: idA, name: "Alpha", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
		{id: idB, name: "alpha", currentStock: 5, minimumStock: 10, averageDailySales: "2", leadTimeDays: 0, criticalityLevel: 3},
		{id: idC, name: "Gamma", currentStock: -1, minimumStock: 4, averageDailySales: "0.5", leadTimeDays: 2, criticalityLevel: 3},
	}

	forward := make([]*domain.Part, 0, len(specs))
	for _, opts := range specs {
		forward = append(forward, newPart(opts))
	}

	reversed := make([]*domain.Part, 0, len(specs))
	for i := len(specs) - 1; i >= 0; i-- {
		reversed = append(reversed, newPart(specs[i]))
	}

	engine := domain.NewPriorityEngine()
	first := engine.CalculateAndRank(forward)
	again := engine.CalculateAndRank(forward)
	fromReversed := engine.CalculateAndRank(reversed)

	ids := func(items []domain.PriorityItem) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			out = append(out, item.PartID)
		}
		return out
	}

	require.Len(t, first, 3)
	assert.Equal(t, ids(first), ids(again), "repeated calls must agree")
	assert.Equal(t, ids(first), ids(fromReversed), "input order must not affect the ranking")
}

// TestPriorityEngine_DoesNotMutateInput guards the purity the design depends on.
func TestPriorityEngine_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	part := newPart(partOpts{name: "Filter", currentStock: 15, minimumStock: 20, averageDailySales: "4", leadTimeDays: 5, criticalityLevel: 3})
	before := *part

	domain.NewPriorityEngine().CalculateAndRank([]*domain.Part{part})

	assert.Equal(t, before.CurrentStock, part.CurrentStock)
	assert.Equal(t, before.MinimumStock, part.MinimumStock)
	assert.Equal(t, before.Name, part.Name)
	assert.True(t, before.AverageDailySales.Equal(part.AverageDailySales))
}
