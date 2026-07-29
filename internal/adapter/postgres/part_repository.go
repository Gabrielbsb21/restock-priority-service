package postgres

import (
	"context"
	"errors"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var _ application.PartRepository = (*PartRepository)(nil)

// mutableColumns are the columns a full replacement is allowed to write.
//
// Naming them explicitly is required, not stylistic: Updates with a struct skips
// zero-valued fields, so a full replacement setting currentStock, minimumStock or
// leadTimeDays to 0 would silently keep the previous value. Listing a column makes
// GORM write it even when it is zero. id and created_at are deliberately absent;
// updated_at is absent because autoUpdateTime sets it regardless of this list.
var mutableColumns = []string{
	"name",
	"category",
	"current_stock",
	"minimum_stock",
	"average_daily_sales",
	"lead_time_days",
	"unit_cost",
	"criticality_level",
}

type PartRepository struct {
	db *gorm.DB
}

func NewPartRepository(db *gorm.DB) *PartRepository {
	return &PartRepository{db: db}
}

func (r *PartRepository) Create(ctx context.Context, part *domain.Part) error {
	model := FromDomain(part)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *PartRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Part, error) {
	var model PartModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrPartNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.ToDomain(), nil
}

func (r *PartRepository) List(ctx context.Context, filter application.ListFilter) ([]*domain.Part, int, error) {
	// Each finisher gets a fresh statement. Reusing one *gorm.DB across Count and
	// Find lets conditions from the first call leak into the second.
	filtered := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&PartModel{})
		if filter.Category != "" {
			query = query.Where("category = ?", filter.Category)
		}
		return query
	}

	var total int64
	if err := filtered().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []PartModel
	err := filtered().
		Order("LOWER(name) ASC").
		Order("id ASC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	parts := make([]*domain.Part, len(models))
	for i, m := range models {
		parts[i] = m.ToDomain()
	}

	return parts, int(total), nil
}

func (r *PartRepository) Update(ctx context.Context, part *domain.Part) error {
	model := FromDomain(part)
	res := r.db.WithContext(ctx).
		Model(&PartModel{}).
		Where("id = ?", part.ID).
		Select(mutableColumns).
		Updates(model)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return application.ErrPartNotFound
	}
	return nil
}

func (r *PartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&PartModel{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return application.ErrPartNotFound
	}
	return nil
}

func (r *PartRepository) ListAll(ctx context.Context) ([]*domain.Part, error) {
	var models []PartModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}

	parts := make([]*domain.Part, len(models))
	for i, m := range models {
		parts[i] = m.ToDomain()
	}
	return parts, nil
}
