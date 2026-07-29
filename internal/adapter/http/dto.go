package http

import (
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// jsonDecimal serializes an exact decimal as a JSON number.
//
// decimal.Decimal marshals to a quoted string by default, which the API contract
// forbids. Wrapping it keeps the fix in the transport layer, rather than flipping
// the library's package-level MarshalJSONWithoutQuotes global — that would be
// mutable package state and would not hold inside tests that never set it.
type jsonDecimal struct {
	decimal.Decimal
}

func (d jsonDecimal) MarshalJSON() ([]byte, error) {
	return []byte(d.String()), nil
}

type PartWriteRequest struct {
	Name              *string          `json:"name"`
	Category          *string          `json:"category"`
	CurrentStock      *int64           `json:"currentStock"`
	MinimumStock      *int64           `json:"minimumStock"`
	AverageDailySales *decimal.Decimal `json:"averageDailySales"`
	LeadTimeDays      *int32           `json:"leadTimeDays"`
	UnitCost          *decimal.Decimal `json:"unitCost"`
	CriticalityLevel  *int             `json:"criticalityLevel"`
}

func (r *PartWriteRequest) ToDomain() (*domain.Part, domain.FieldErrors) {
	errs := make(domain.FieldErrors)

	if r.Name == nil {
		errs["name"] = "is required"
	}
	if r.Category == nil {
		errs["category"] = "is required"
	}
	if r.CurrentStock == nil {
		errs["currentStock"] = "is required"
	}
	if r.MinimumStock == nil {
		errs["minimumStock"] = "is required"
	}
	if r.AverageDailySales == nil {
		errs["averageDailySales"] = "is required"
	}
	if r.LeadTimeDays == nil {
		errs["leadTimeDays"] = "is required"
	}
	if r.UnitCost == nil {
		errs["unitCost"] = "is required"
	}
	if r.CriticalityLevel == nil {
		errs["criticalityLevel"] = "is required"
	}

	if len(errs) > 0 {
		return nil, errs
	}

	part := &domain.Part{
		Name:              *r.Name,
		Category:          *r.Category,
		CurrentStock:      *r.CurrentStock,
		MinimumStock:      *r.MinimumStock,
		AverageDailySales: *r.AverageDailySales,
		LeadTimeDays:      *r.LeadTimeDays,
		UnitCost:          *r.UnitCost,
		CriticalityLevel:  *r.CriticalityLevel,
	}

	return part, nil
}

type PartResponse struct {
	ID                uuid.UUID   `json:"id"`
	Name              string      `json:"name"`
	Category          string      `json:"category"`
	CurrentStock      int64       `json:"currentStock"`
	MinimumStock      int64       `json:"minimumStock"`
	AverageDailySales jsonDecimal `json:"averageDailySales"`
	LeadTimeDays      int32       `json:"leadTimeDays"`
	UnitCost          jsonDecimal `json:"unitCost"`
	CriticalityLevel  int         `json:"criticalityLevel"`
}

func NewPartResponse(p *domain.Part) PartResponse {
	return PartResponse{
		ID:                p.ID,
		Name:              p.Name,
		Category:          p.Category,
		CurrentStock:      p.CurrentStock,
		MinimumStock:      p.MinimumStock,
		AverageDailySales: jsonDecimal{p.AverageDailySales},
		LeadTimeDays:      p.LeadTimeDays,
		UnitCost:          jsonDecimal{p.UnitCost},
		CriticalityLevel:  p.CriticalityLevel,
	}
}

type PaginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type ListPartsResponse struct {
	Items      []PartResponse `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
}

// PriorityItemResponse is the wire shape of a ranked restock candidate. The
// contract is exactly these six fields: category, criticality, sales, unit cost and
// expected consumption are deliberately absent.
type PriorityItemResponse struct {
	PartID         uuid.UUID   `json:"partId"`
	Name           string      `json:"name"`
	CurrentStock   int64       `json:"currentStock"`
	ProjectedStock jsonDecimal `json:"projectedStock"`
	MinimumStock   int64       `json:"minimumStock"`
	UrgencyScore   jsonDecimal `json:"urgencyScore"`
}

type PriorityListResponse struct {
	Priorities []PriorityItemResponse `json:"priorities"`
}

// NewPriorityListResponse always builds a non-nil slice so an empty ranking
// serializes as [] rather than null.
func NewPriorityListResponse(items []domain.PriorityItem) PriorityListResponse {
	priorities := make([]PriorityItemResponse, 0, len(items))
	for _, item := range items {
		priorities = append(priorities, PriorityItemResponse{
			PartID:         item.PartID,
			Name:           item.Name,
			CurrentStock:   item.CurrentStock,
			ProjectedStock: jsonDecimal{item.ProjectedStock},
			MinimumStock:   item.MinimumStock,
			UrgencyScore:   jsonDecimal{item.UrgencyScore},
		})
	}

	return PriorityListResponse{Priorities: priorities}
}
