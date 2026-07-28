package postgres

import (
	"context"
	"errors"

	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	query := r.db.WithContext(ctx).Model(&PartModel{})

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []PartModel
	err := query.
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
	res := r.db.WithContext(ctx).Model(&PartModel{}).Where("id = ?", part.ID).Updates(model)
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
