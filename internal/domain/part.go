package domain

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type FieldErrors map[string]string

func (fe FieldErrors) Error() string {
	var msgs []string
	for k, v := range fe {
		msgs = append(msgs, k+": "+v)
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
	}

	if p.LeadTimeDays < 0 {
		errs["leadTimeDays"] = "must be greater than or equal to zero"
	}

	if p.UnitCost.IsNegative() {
		errs["unitCost"] = "must be greater than or equal to zero"
	} else if p.UnitCost.Exponent() < -2 {
		errs["unitCost"] = "must have at most two fractional digits"
	}

	if p.CriticalityLevel < 1 || p.CriticalityLevel > 5 {
		errs["criticalityLevel"] = "must be between 1 and 5"
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
