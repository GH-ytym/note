package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"

	"gorm.io/gorm"
)

// Repository 声明 Event Service 需要的数据库操作。
type Repository interface {
	Create(ctx context.Context, item *model.Event) error
	List(ctx context.Context, query ListQuery) ([]model.Event, int64, error)
	ListInRange(ctx context.Context, userID uint, from, to time.Time) ([]model.Event, error)
	Get(ctx context.Context, id, userID uint) (model.Event, error)
	Update(ctx context.Context, item *model.Event, version, userID uint) error
	Delete(ctx context.Context, id, userID uint) error
	PatchRole(ctx context.Context, actorID, eventID uint, userIDs []uint, role model.EventRole) error
}

// gormRepository 是 Repository 的 GORM 实现，对 event 包外隐藏。
type gormRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Get(ctx context.Context, id, userID uint) (model.Event, error) {
	var item model.Event
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadEventForUser(tx, id, userID); err != nil {
			return err
		}
		return eventDetails(tx).Preload("Members").First(&item, id).Error
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.Event{}, err
	}
	return item, nil
}

func (r *gormRepository) Update(ctx context.Context, item *model.Event, version, userID uint) error {
	normalizeEventTimes(item)
	var updated model.Event
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		target, err := loadEventForUser(tx, item.ID, userID)
		if err != nil {
			return err
		}
		if target.CreatorID != userID {
			var count int64
			if err := tx.Model(&model.EventMember{}).
				Where("event_id = ? AND user_id = ? AND role = ?", item.ID, userID, model.EventEditor).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return apperrors.ErrEventEditDenied
			}
		}
		result := tx.
			Model(&model.Event{}).
			Where("id = ? AND version = ?", item.ID, version).
			Updates(map[string]any{
				"title":     item.Title,
				"content":   item.Content,
				"starts_at": item.StartsAt,
				"ends_at":   item.EndsAt,
				"version":   version + 1,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return apperrors.ErrEventConcurrentUpdate
		}
		// 更新字段仍与原 Event 一致，不改所属群、创建者或重复日期。
		return eventDetails(tx).First(&updated, item.ID).Error
	})
	if err != nil {
		return err
	}
	*item = updated
	return nil
}

func (r *gormRepository) Create(
	ctx context.Context,
	item *model.Event,
) error {
	normalizeEventTimes(item)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var members []model.GroupMember
		if err := tx.Select("user_id").Where("group_id = ?", item.GroupID).Find(&members).Error; err != nil {
			return err
		}
		creatorPresent := false
		for _, member := range members {
			if member.UserID == item.CreatorID {
				creatorPresent = true
				break
			}
		}
		if !creatorPresent {
			return apperrors.ErrGroupAccessDenied
		}
		if err := tx.Omit("Group", "Creator", "Members").Create(item).Error; err != nil {
			return fmt.Errorf("create event: %w", err)
		}
		item.Members = make([]model.EventMember, 0, len(members))
		for _, member := range members {
			role := model.EventViewer
			if member.UserID == item.CreatorID {
				role = model.EventEditor
			}
			item.Members = append(item.Members, model.EventMember{EventID: item.ID, UserID: member.UserID, Role: role})
		}
		if err := tx.Omit("User").CreateInBatches(&item.Members, 200).Error; err != nil {
			return fmt.Errorf("initialize event permissions: %w", err)
		}
		return tx.Select("id", "username", "suffix", "nickname", "avatar").First(&item.Creator, item.CreatorID).Error
	})
}

// SQLite compares these stored datetime strings in the range CHECK constraint.
// Normalize both endpoints before every write, including patches sent in UTC.
func normalizeEventTimes(item *model.Event) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	item.StartsAt = item.StartsAt.In(location)
	item.EndsAt = item.EndsAt.In(location)
}

func (r *gormRepository) ListInRange(
	ctx context.Context,
	userID uint,
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

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Preload("Members", "user_id = ?", userID).
			Where(`EXISTS (SELECT 1 FROM group_members gm
			WHERE gm.group_id = events.group_id AND gm.user_id = ?)`, userID).
			Preload("CustomDates").
			Where(`
			((
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
	}, &sql.TxOptions{ReadOnly: true})

	if err != nil {
		return nil, fmt.Errorf("list event candidates: %w", err)
	}

	return items, nil
}

// 成员资格与日程来自同一事务；旧 EventMember 不能代替当前群成员关系。
func loadEventForUser(tx *gorm.DB, id, userID uint) (model.Event, error) {
	var item model.Event
	err := tx.Select("id", "group_id", "creator_id").First(&item, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, apperrors.ErrEventNotFound
	}
	if err != nil {
		return item, err
	}
	if err := checkGroupMember(tx, item.GroupID, userID); err != nil {
		return model.Event{}, err
	}
	return item, nil
}

func checkGroupMember(tx *gorm.DB, groupID, userID uint) error {
	var count int64
	if err := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return apperrors.ErrGroupAccessDenied
	}
	return nil
}

func eventDetails(db *gorm.DB) *gorm.DB {
	return db.Preload("CustomDates").Preload("Creator", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "suffix", "nickname", "avatar")
	})
}

func (r *gormRepository) List(ctx context.Context, query ListQuery) ([]model.Event, int64, error) {
	items := make([]model.Event, 0)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := checkGroupMember(tx, query.GroupID, query.UserID); err != nil {
			return err
		}
		if err := tx.Model(&model.Event{}).Where("group_id = ?", query.GroupID).Count(&total).Error; err != nil {
			return err
		}
		return eventDetails(tx).Where("group_id = ?", query.GroupID).Order("id DESC").
			Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Find(&items).Error
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *gormRepository) Delete(ctx context.Context, id, userID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := loadEventForUser(tx, id, userID)
		if err != nil {
			return err
		}
		if item.CreatorID != userID {
			return apperrors.ErrEventDeleteDenied
		}
		if err := tx.Where("event_id = ?", id).Delete(&model.EventDate{}).Error; err != nil {
			return err
		}
		if err := tx.Where("event_id = ?", id).Delete(&model.EventMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Event{}, id).Error
	})
}

func (r *gormRepository) PatchRole(ctx context.Context, actorID, eventID uint, userIDs []uint, role model.EventRole) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := loadEventForUser(tx, eventID, actorID)
		if err != nil {
			return err
		}
		if item.CreatorID != actorID {
			return apperrors.ErrEventPermissionDenied
		}
		if role == model.EventViewer && slices.Contains(userIDs, item.CreatorID) {
			return apperrors.ErrEventRoleInvalid
		}
		var members, permissions int64
		if err := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id IN ?", item.GroupID, userIDs).Count(&members).Error; err != nil {
			return err
		}
		if members != int64(len(userIDs)) {
			return apperrors.ErrEventMemberNotFound
		}
		if err := tx.Model(&model.EventMember{}).Where("event_id = ? AND user_id IN ?", eventID, userIDs).Count(&permissions).Error; err != nil {
			return err
		}
		if permissions != int64(len(userIDs)) {
			return apperrors.ErrEventMemberNotFound
		}
		return tx.Model(&model.EventMember{}).Where("event_id = ? AND user_id IN ?", eventID, userIDs).Update("role", role).Error
	})
}
