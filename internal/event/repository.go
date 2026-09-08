package event

import (
	"context"
	"fmt"

	"note/internal/model"

	"gorm.io/gorm"
)

// EventRepository 声明 Event Service 需要的数据库操作。
type EventRepository interface {
	Create(ctx context.Context, item *model.Event) error
}

// gormRepository 是 EventRepository 的 GORM 实现。
type gormRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) EventRepository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(
	ctx context.Context,
	item *model.Event,
) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	return nil
}
