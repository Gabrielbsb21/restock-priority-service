package domain

import (
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// The decimal fields are bounded above as well as below, and the bound counts digits
// rather than comparing values.
//
// decimal.Decimal stores a coefficient and an exponent, so "1e10000000" costs ten
// bytes to accept and ten megabytes to render — and it is rendered on every write,
// every response, and every ranking pass. A single such row is enough to make
// GET /restock/priorities unusable. Counting digits reads the exponent instead of
// materializing the number, so the guard itself stays cheap.
//
// maxIntegerDigits is unit_cost's own column limit: NUMERIC(15, 2) holds thirteen
// digits before the point. What the column would have rejected as a write error now
// fails validation as a field error.
const (
	maxIntegerDigits            = 13
	maxUnitCostFractionalDigits = 2
	maxSalesFractionalDigits    = 6
)

// FieldErrors reports every invalid field of a single request at once, keyed by the
// field name used on the wire.
type FieldErrors map[string]string

// Error renders the fields in sorted order so the message is stable. Ranging over
// the map directly would produce a different string on every call.
func (fe FieldErrors) Error() string {
	names := make([]string, 0, len(fe))
	for name := range fe {
		names = append(names, name)
	}
	sort.Strings(names)

	msgs := make([]string, 0, len(names))
	for _, name := range names {
		msgs = append(msgs, name+": "+fe[name])
	}

	return strings.Join(msgs, "; ")
}

type Part struct {
	ID                uuid.UUID
	Name              string
	Category          string
	CurrentStock      int64
	MinimumStock      int64
	AverageDailySales decimal.Decimal
	LeadTimeDays      int32
	UnitCost          decimal.Decimal
	CriticalityLevel  int
}

func (p *Part) Validate() error {
	errs := make(FieldErrors)

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		errs["name"] = "must not be empty"
	} else if len([]rune(p.Name)) > 200 {
		errs["name"] = "must not exceed 200 characters"
	}

	p.Category = strings.TrimSpace(p.Category)
	if p.Category == "" {
		errs["category"] = "must not be empty"
	} else if len([]rune(p.Category)) > 100 {
		errs["category"] = "must not exceed 100 characters"
	}

	if p.MinimumStock < 0 {
		errs["minimumStock"] = "must be greater than or equal to zero"
	}

	if p.AverageDailySales.IsNegative() {
		errs["averageDailySales"] = "must be greater than or equal to zero"
	} else if msg := decimalRangeError(p.AverageDailySales, maxSalesFractionalDigits, "must have at most six fractional digits"); msg != "" {
		errs["averageDailySales"] = msg
	}

	if p.LeadTimeDays < 0 {
		errs["leadTimeDays"] = "must be greater than or equal to zero"
	}

	if p.UnitCost.IsNegative() {
		errs["unitCost"] = "must be greater than or equal to zero"
	} else if msg := decimalRangeError(p.UnitCost, maxUnitCostFractionalDigits, "must have at most two fractional digits"); msg != "" {
		errs["unitCost"] = msg
	}

	if p.CriticalityLevel < 1 || p.CriticalityLevel > 5 {
		errs["criticalityLevel"] = "must be between 1 and 5"
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// decimalRangeError reports why d is out of range, or an empty string when it is
// acceptable. The scale is checked first because the exponent alone settles it, and
// the caller supplies that message so each field keeps its own wording.
func decimalRangeError(d decimal.Decimal, maxFractionalDigits int, scaleMessage string) string {
	if int(d.Exponent()) < -maxFractionalDigits {
		return scaleMessage
	}

	if integerDigits(d) > maxIntegerDigits {
		return "must not have more than " + strconv.Itoa(maxIntegerDigits) + " digits before the decimal point"
	}

	return ""
}

// integerDigits reports how many digits sit before the decimal point, derived from
// the length of the coefficient and the exponent. A value below one is reported as a
// single digit, which is what "0.5" occupies on the wire.
func integerDigits(d decimal.Decimal) int {
	digits := d.NumDigits() + int(d.Exponent())
	if digits < 1 {
		return 1
	}

	return digits
}
