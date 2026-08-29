package todo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"note/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 声明 Service 所需的数据库操作。
type Repository interface {
	Create(ctx context.Context, item *model.Todo) error
	List(ctx context.Context, query ListQuery) ([]model.Todo, int64, error)
	//在指定时间段找所有可能出现的Todo（只缩小范围不保证全对）
	CalendarCandidates(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]model.Todo, error)
	ByID(ctx context.Context, id uint) (model.Todo, error)
	Patch(ctx context.Context, id uint, command PatchCommand) (model.Todo, error)
	Delete(ctx context.Context, id uint) error
	//给单个todo的某一天设置完成状态
	SetOccurrenceDone(
		ctx context.Context,
		todoID uint,
		occursOn time.Time,
		done bool,
	) error
	//查询某段时间范围内的todo完成情况
	CompletionsInRange(
		ctx context.Context,
		todoIDs []uint,
		from time.Time,
		to time.Time,
	) ([]model.TodoCompletion, error)
}

// gormRepository 是 Repository 的 GORM 实现，对 todo 包外隐藏。
type gormRepository struct {
	db *gorm.DB
}

func (r *gormRepository) CalendarCandidates(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]model.Todo, error) {
	items := make([]model.Todo, 0)
	fromDate := from.Format(time.DateOnly)
	toDate := to.Format(time.DateOnly)

	err := r.db.
		WithContext(ctx).
		Preload(
			//custom模式
			"CustomDates",
			"date >= ? AND date < ?",
			fromDate,
			toDate,
		).
		Where(`
		starts_at IS NOT NULL
		AND (
			(
				repeat_mode = ?
				AND starts_at >= ?
				AND starts_at < ?
			)
			OR
			(
				repeat_mode NOT IN (?, ?)
				AND starts_at < ?
			)
			OR
			(
				repeat_mode = ?
				AND EXISTS (
					SELECT 1
					FROM todo_dates
					WHERE todo_dates.todo_id = todos.id
						AND todo_dates.date >= ?
						AND todo_dates.date < ?
				)
			)
		)
	`,
			model.RepeatOnce,
			from,
			to,

			model.RepeatOnce,
			model.RepeatCustom,
			to,

			model.RepeatCustom,
			fromDate,
			toDate,
		).
		Order("starts_at ASC").
		Find(&items).
		Error

	if err != nil {
		return nil, fmt.Errorf("list calendar candidates: %w", err)
	}

	return items, nil
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, item *model.Todo) error {
	err := r.db.WithContext(ctx).Create(item).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrContentConflict
	}
	if err != nil {
		return fmt.Errorf("create todo: %w", err)
	}

	return nil
}

func (r *gormRepository) List(ctx context.Context, query ListQuery) ([]model.Todo, int64, error) {
	db := r.db.WithContext(ctx)

	var total int64
	if err := db.Model(&model.Todo{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count todos: %w", err)
	}

	items := make([]model.Todo, 0)
	offset := (query.Page - 1) * query.PageSize
	if err := db.
		Order("id DESC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list todos: %w", err)
	}

	return items, total, nil
}

func (r *gormRepository) ByID(ctx context.Context, id uint) (model.Todo, error) {
	var item model.Todo

	err := r.db.
		WithContext(ctx).
		//service需要自定义日期所以要preload
		Preload("CustomDates").
		First(&item, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Todo{}, ErrNotFound
	}

	if err != nil {
		return model.Todo{}, fmt.Errorf("find todo %d: %w", id, err)
	}

	return item, nil
}

func (r *gormRepository) Patch(
	ctx context.Context,
	id uint,
	command PatchCommand,
) (model.Todo, error) {
	updates := map[string]any{
		//乐观锁
		"version": gorm.Expr("version + 1"),
	}

	if command.Content != nil {
		updates["content"] = *command.Content
	}
	if command.Color != nil {
		updates["color"] = *command.Color
	}
	if command.StartsAt != nil {
		updates["starts_at"] = *command.StartsAt
	}
	if command.RepeatMode != nil {
		updates["repeat_mode"] = *command.RepeatMode
	}
	if command.NotifyMode != nil {
		updates["notify_mode"] = *command.NotifyMode
	}
	if command.AllDone != nil {
		updates["all_done"] = *command.AllDone
	}

	db := r.db.WithContext(ctx)
	var item model.Todo

	//进入事务
	//先用乐观锁更新todo
	err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Todo{}).Where("id=? AND version=?", id, command.Version).Updates(updates)
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return ErrContentConflict
		}
		if res.Error != nil {
			return fmt.Errorf(
				"patch todo %d: %w",
				id,
				res.Error,
			)
		}

		//没匹配上
		if res.RowsAffected == 0 {
			var cnt int64
			if err := tx.
				Model(&model.Todo{}).
				Where("id = ?", id).
				Count(&cnt).
				Error; err != nil {
				return fmt.Errorf(
					"check todo %d after failed patch: %w",
					id,
					err,
				)
			}

			//1.id不对
			if cnt == 0 {
				return ErrNotFound
			}
			//id对还查不到只能是version不对，说明有并发更新
			return ErrConcurrentUpdate
		}

		// 用自定义日期替换旧集合
		if command.CustomDates != nil {
			if err := tx.
				Where("todo_id = ?", id).
				Delete(&model.TodoDate{}).
				Error; err != nil {
				return fmt.Errorf(
					"delete custom dates for todo %d: %w",
					id,
					err,
				)
			}

			dates := make([]model.TodoDate, 0, len(*command.CustomDates))
			for _, date := range *command.CustomDates {
				dates = append(dates, model.TodoDate{
					TodoID: id,
					Date:   date,
				})
			}

			if len(dates) > 0 {
				//批量插入
				if err := tx.Create(&dates).Error; err != nil {
					return fmt.Errorf(
						"create custom dates for todo %d: %w",
						id,
						err,
					)
				}
			}
		}

		// 返回更新后的完整 Todo，包括一对一提醒和一对多自定义日期。
		if err := tx.
			Preload("CustomDates").
			First(&item, id).
			Error; err != nil {
			return fmt.Errorf(
				"load patched todo %d: %w",
				id,
				err,
			)
		}
		return nil
	})
	if err != nil {
		return model.Todo{}, err
	}

	return item, nil
}

func (r *gormRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Todo{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete todo %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *gormRepository) SetOccurrenceDone(
	ctx context.Context,
	todoID uint,
	occursOn time.Time,
	done bool,
) error {
	db := r.db.WithContext(ctx)
	//未完成-完成：写入todocompletion
	if done {
		completion := model.TodoCompletion{
			TodoID:      todoID,
			OccursOn:    occursOn,
			CompletedAt: time.Now(),
		}
		return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&completion).Error
	}
	//完成-未完成：删除todocompletion对应记录
	return db.Where(
		"todo_id = ? AND occurs_on = ?",
		todoID,
		occursOn,
	).Delete(&model.TodoCompletion{}).Error
}

// 一次性查询当前范围的完成记录
func (r *gormRepository) CompletionsInRange(
	ctx context.Context,
	todoIDs []uint,
	from time.Time,
	to time.Time,
) ([]model.TodoCompletion, error) {
	if len(todoIDs) == 0 {
		return []model.TodoCompletion{}, nil
	}

	var completions []model.TodoCompletion
	fromDate := from.Format(time.DateOnly)
	toDate := to.Format(time.DateOnly)

	err := r.db.WithContext(ctx).
		Where(
			"todo_id IN ? AND occurs_on >= ? AND occurs_on < ?",
			todoIDs,
			fromDate,
			toDate,
		).
		Find(&completions).
		Error
	if err != nil {
		return nil, fmt.Errorf("list todo completions in range: %w", err)
	}

	return completions, nil
}
