package event

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"

	"gorm.io/gorm"
)

// EventRepository 声明 Event Service 需要的数据库操作。
type EventRepository interface {
	Create(ctx context.Context, item *model.Event) error
	ListInRange(ctx context.Context, from, to time.Time) ([]model.Event, error)
	Get(ctx context.Context, id uint) (model.Event, error)
	Update(ctx context.Context, item *model.Event, version uint) error
}

func (r *gormRepository) Get(ctx context.Context, id uint) (model.Event, error) {
	var item model.Event
	err := r.db.WithContext(ctx).Preload("CustomDates").First(&item, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, apperrors.ErrEventNotFound
	}
	return item, err
}

func (r *gormRepository) Update(ctx context.Context, item *model.Event, version uint) error {
	result := r.db.WithContext(ctx).Model(&model.Event{}).Where("id = ? AND version = ?", item.ID, version).Updates(map[string]interface{}{
		"title": item.Title, "content": item.Content, "starts_at": item.StartsAt, "ends_at": item.EndsAt, "version": version + 1,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrEventConcurrentUpdate
	}
	item.Version = version + 1
	return nil
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

func (r *gormRepository) ListInRange(
	ctx context.Context,
	from, to time.Time,
) ([]model.Event, error) {
	items := make([]model.Event, 0)

	repeatModes := []model.RepeatMode{
		model.RepeatDaily,
		model.RepeatWeekdays,
		model.RepeatWeekends,
		model.RepeatWeekly,
		model.RepeatMonthly,
	}

	err := r.db.WithContext(ctx).
		Preload("CustomDates").
		Where(`
			(
				repeat_mode = ?
				AND starts_at < ?
				AND ends_at > ?
			)
			OR
			(
				repeat_mode IN ?
				AND starts_at < ?
			)
			OR
			(
				repeat_mode = ?
			)
		`,
			model.RepeatOnce, to, from,
			repeatModes, to,
			model.RepeatCustom,
		).
		Order("starts_at ASC").
		Order("id ASC").
		Find(&items).
		Error

	if err != nil {
		return nil, fmt.Errorf("list event candidates: %w", err)
	}

	return items, nil
}
