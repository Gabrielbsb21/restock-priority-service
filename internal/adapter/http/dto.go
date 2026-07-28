package http

import (
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	ID                uuid.UUID       `json:"id"`
	Name              string          `json:"name"`
	Category          string          `json:"category"`
	CurrentStock      int64           `json:"currentStock"`
	MinimumStock      int64           `json:"minimumStock"`
	AverageDailySales decimal.Decimal `json:"averageDailySales"`
	LeadTimeDays      int32           `json:"leadTimeDays"`
	UnitCost          decimal.Decimal `json:"unitCost"`
	CriticalityLevel  int             `json:"criticalityLevel"`
}

func NewPartResponse(p *domain.Part) PartResponse {
	return PartResponse{
		ID:                p.ID,
		Name:              p.Name,
		Category:          p.Category,
		CurrentStock:      p.CurrentStock,
		MinimumStock:      p.MinimumStock,
		AverageDailySales: p.AverageDailySales,
		LeadTimeDays:      p.LeadTimeDays,
		UnitCost:          p.UnitCost,
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

type PriorityListResponse struct {
	Priorities []domain.PriorityItem `json:"priorities"`
}
