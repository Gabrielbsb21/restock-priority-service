package postgres

import (
	"time"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PartModel struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Name              string          `gorm:"type:varchar(200);not null"`
	Category          string          `gorm:"type:varchar(100);not null;index:idx_parts_category"`
	CurrentStock      int64           `gorm:"not null"`
	MinimumStock      int64           `gorm:"not null"`
	AverageDailySales decimal.Decimal `gorm:"type:numeric;not null"`
	LeadTimeDays      int32           `gorm:"not null"`
	UnitCost          decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	CriticalityLevel  int             `gorm:"not null"`
	CreatedAt         time.Time       `gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `gorm:"autoUpdateTime"`
}

func (PartModel) TableName() string {
	return "parts"
}

func (m *PartModel) ToDomain() *domain.Part {
	return &domain.Part{
		ID:                m.ID,
		Name:              m.Name,
		Category:          m.Category,
		CurrentStock:      m.CurrentStock,
		MinimumStock:      m.MinimumStock,
		AverageDailySales: m.AverageDailySales,
		LeadTimeDays:      m.LeadTimeDays,
		UnitCost:          m.UnitCost,
		CriticalityLevel:  m.CriticalityLevel,
	}
}

func FromDomain(p *domain.Part) *PartModel {
	return &PartModel{
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
