package domain_test

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validPart is the baseline every field-level case mutates one field of.
func validPart() *domain.Part {
	return newPart(partOpts{
		name:              "Spark Plug",
		currentStock:      10,
		minimumStock:      15,
		averageDailySales: "2.5",
		leadTimeDays:      3,
		unitCost:          "4.99",
		criticalityLevel:  4,
	})
}

// TestPart_Validate_FieldRules walks each invariant, including the boundary on both
// sides, so an off-by-one in a comparison cannot pass.
func TestPart_Validate_FieldRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(p *domain.Part)
		wantField string
	}{
		{name: "baseline is valid", mutate: func(*domain.Part) {}},

		{name: "empty name", mutate: func(p *domain.Part) { p.Name = "" }, wantField: "name"},
		{name: "whitespace-only name", mutate: func(p *domain.Part) { p.Name = "   " }, wantField: "name"},
		{name: "name at the 200 character limit", mutate: func(p *domain.Part) { p.Name = strings.Repeat("a", 200) }},
		{name: "name past the 200 character limit", mutate: func(p *domain.Part) { p.Name = strings.Repeat("a", 201) }, wantField: "name"},
		{
			// Counted in runes, not bytes: 200 accented characters are 400 bytes.
			name:   "name of 200 multi-byte characters",
			mutate: func(p *domain.Part) { p.Name = strings.Repeat("á", 200) },
		},
		{
			name:      "name of 201 multi-byte characters",
			mutate:    func(p *domain.Part) { p.Name = strings.Repeat("á", 201) },
			wantField: "name",
		},

		{name: "empty category", mutate: func(p *domain.Part) { p.Category = "" }, wantField: "category"},
		{name: "whitespace-only category", mutate: func(p *domain.Part) { p.Category = "\t\n " }, wantField: "category"},
		{name: "category at the 100 character limit", mutate: func(p *domain.Part) { p.Category = strings.Repeat("b", 100) }},
		{name: "category past the 100 character limit", mutate: func(p *domain.Part) { p.Category = strings.Repeat("b", 101) }, wantField: "category"},

		// BR-010: negative current stock is a legitimate recorded state.
		{name: "negative current stock is accepted", mutate: func(p *domain.Part) { p.CurrentStock = -500 }},
		{name: "minimum int64 current stock is accepted", mutate: func(p *domain.Part) { p.CurrentStock = math.MinInt64 }},

		{name: "zero minimum stock", mutate: func(p *domain.Part) { p.MinimumStock = 0 }},
		{name: "negative minimum stock", mutate: func(p *domain.Part) { p.MinimumStock = -1 }, wantField: "minimumStock"},

		{name: "zero sales", mutate: func(p *domain.Part) { p.AverageDailySales = decimal.Zero }},
		{
			name:      "negative sales",
			mutate:    func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("-0.5") },
			wantField: "averageDailySales",
		},
		{
			name:   "sales with many fractional digits is accepted",
			mutate: func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("0.123456") },
		},
		{
			name:      "sales with one fractional digit too many",
			mutate:    func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("0.1234567") },
			wantField: "averageDailySales",
		},
		{
			name:   "sales at the integer digit limit",
			mutate: func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("9999999999999") },
		},
		{
			name:      "sales one integer digit past the limit",
			mutate:    func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("10000000000000") },
			wantField: "averageDailySales",
		},
		{
			// BR-015. The coefficient is a single digit, so nothing but the exponent
			// reveals that rendering this value would produce ten megabytes.
			name:      "sales carrying a huge positive exponent",
			mutate:    func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("1e10000000") },
			wantField: "averageDailySales",
		},
		{
			name:      "sales carrying a huge negative exponent",
			mutate:    func(p *domain.Part) { p.AverageDailySales = decimal.RequireFromString("1e-10000000") },
			wantField: "averageDailySales",
		},

		{name: "zero lead time", mutate: func(p *domain.Part) { p.LeadTimeDays = 0 }},
		{name: "negative lead time", mutate: func(p *domain.Part) { p.LeadTimeDays = -2 }, wantField: "leadTimeDays"},
		{name: "maximum lead time", mutate: func(p *domain.Part) { p.LeadTimeDays = math.MaxInt32 }},

		{name: "zero unit cost", mutate: func(p *domain.Part) { p.UnitCost = decimal.Zero }},
		{
			name:      "negative unit cost",
			mutate:    func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("-0.01") },
			wantField: "unitCost",
		},
		{name: "unit cost with two fractional digits", mutate: func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("10.99") }},
		{
			name:      "unit cost with three fractional digits",
			mutate:    func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("10.999") },
			wantField: "unitCost",
		},
		{
			// The rule is checked on the decimal's exponent, so a trailing zero counts
			// as a third fractional digit even though the value equals 10.12.
			name:      "unit cost with a trailing zero in a third decimal place",
			mutate:    func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("10.120") },
			wantField: "unitCost",
		},
		{name: "whole unit cost", mutate: func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("18") }},
		{
			// The largest value NUMERIC(15, 2) can hold.
			name:   "unit cost at the integer digit limit",
			mutate: func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("9999999999999.99") },
		},
		{
			name:      "unit cost one integer digit past the limit",
			mutate:    func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("10000000000000") },
			wantField: "unitCost",
		},
		{
			name:      "unit cost carrying a huge positive exponent",
			mutate:    func(p *domain.Part) { p.UnitCost = decimal.RequireFromString("1e10000000") },
			wantField: "unitCost",
		},

		{name: "criticality below the range", mutate: func(p *domain.Part) { p.CriticalityLevel = 0 }, wantField: "criticalityLevel"},
		{name: "negative criticality", mutate: func(p *domain.Part) { p.CriticalityLevel = -1 }, wantField: "criticalityLevel"},
		{name: "criticality at the lower bound", mutate: func(p *domain.Part) { p.CriticalityLevel = 1 }},
		{name: "criticality at the upper bound", mutate: func(p *domain.Part) { p.CriticalityLevel = 5 }},
		{name: "criticality above the range", mutate: func(p *domain.Part) { p.CriticalityLevel = 6 }, wantField: "criticalityLevel"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			part := validPart()
			tc.mutate(part)

			err := part.Validate()

			if tc.wantField == "" {
				assert.NoError(t, err)
				return
			}

			var fieldErrs domain.FieldErrors
			require.ErrorAs(t, err, &fieldErrs)
			assert.Contains(t, fieldErrs, tc.wantField)
			assert.Len(t, fieldErrs, 1, "only the mutated field should be reported")
		})
	}
}

// TestPart_Validate_RejectsHugeExponentsCheaply is the regression gate for BR-015.
//
// The point of the rule is not only that the value is refused but that refusing it is
// free. Rendering 1e10000000 takes over two seconds and allocates ten megabytes, so a
// guard that reached for the digits — String, Value, a comparison against a literal
// bound — would blow this budget by three orders of magnitude while the digit count
// path costs microseconds. No t.Parallel, so a busy sibling cannot borrow the clock.
func TestPart_Validate_RejectsHugeExponentsCheaply(t *testing.T) {
	const budget = time.Second

	for _, raw := range []string{"1e10000000", "1e-10000000", "-1e10000000"} {
		t.Run(raw, func(t *testing.T) {
			part := validPart()
			part.AverageDailySales = decimal.RequireFromString(raw)
			part.UnitCost = decimal.RequireFromString(raw)

			start := time.Now()
			err := part.Validate()
			elapsed := time.Since(start)

			var fieldErrs domain.FieldErrors
			require.ErrorAs(t, err, &fieldErrs)
			assert.Contains(t, fieldErrs, "averageDailySales")
			assert.Contains(t, fieldErrs, "unitCost")
			assert.Less(t, elapsed, budget, "validation must not materialize the number")
		})
	}
}

// TestPart_Validate_ReportsEveryInvalidField covers the contract that a validation
// error may name more than one field.
func TestPart_Validate_ReportsEveryInvalidField(t *testing.T) {
	t.Parallel()

	part := &domain.Part{
		Name:              "   ",
		Category:          "",
		MinimumStock:      -1,
		AverageDailySales: decimal.RequireFromString("-0.5"),
		LeadTimeDays:      -2,
		UnitCost:          decimal.RequireFromString("10.123"),
		CriticalityLevel:  6,
	}

	err := part.Validate()

	var fieldErrs domain.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Len(t, fieldErrs, 7)
	for _, field := range []string{
		"name", "category", "minimumStock", "averageDailySales", "leadTimeDays", "unitCost", "criticalityLevel",
	} {
		assert.Contains(t, fieldErrs, field)
	}
}

// TestPart_Validate_NormalizesNameAndCategory pins the trimming that the persisted
// and returned representation depends on.
func TestPart_Validate_NormalizesNameAndCategory(t *testing.T) {
	t.Parallel()

	part := validPart()
	part.Name = "  Oil Filter X  "
	part.Category = "\tengine\n"

	require.NoError(t, part.Validate())
	assert.Equal(t, "Oil Filter X", part.Name)
	assert.Equal(t, "engine", part.Category)
}

// TestFieldErrors_ErrorIsStable guards against ranging over the map, which would
// produce a different message on every call.
func TestFieldErrors_ErrorIsStable(t *testing.T) {
	t.Parallel()

	errs := domain.FieldErrors{
		"criticalityLevel":  "must be between 1 and 5",
		"name":              "must not be empty",
		"averageDailySales": "must be greater than or equal to zero",
	}

	first := errs.Error()
	for range 20 {
		assert.Equal(t, first, errs.Error())
	}

	assert.Equal(t,
		"averageDailySales: must be greater than or equal to zero; "+
			"criticalityLevel: must be between 1 and 5; "+
			"name: must not be empty",
		first)
}

// TestFieldErrors_SurvivesWrapping is why the HTTP layer uses errors.As instead of a
// type assertion: a wrapped cause must still map to 400, not 500.
func TestFieldErrors_SurvivesWrapping(t *testing.T) {
	t.Parallel()

	part := validPart()
	part.CriticalityLevel = 9

	wrapped := errors.Join(errors.New("create part"), part.Validate())

	var fieldErrs domain.FieldErrors
	require.ErrorAs(t, wrapped, &fieldErrs)
	assert.Contains(t, fieldErrs, "criticalityLevel")
}
