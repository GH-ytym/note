package notification

import (
	"context"
	"fmt"
	"time"

	"note/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MarkRead 批量标记当前用户选中的通知为已读。
// 已读、已删除或属于其他用户的通知会被忽略。
// 整批操作在同一个事务中完成，只推进一次业务版本。
func (r *gormRepository) MarkRead(
	ctx context.Context,
	uid uint,
	ids []uint,
) error {
	if uid == 0 {
		return fmt.Errorf("invalid user ID")
	}
	if len(ids) == 0 {
		return fmt.Errorf("notification IDs cannot be empty")
	}

	// 去掉重复 ID，避免重复放进 SQL 参数。
	keys := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))

	for _, id := range ids {
		if id == 0 {
			return fmt.Errorf("invalid notification ID")
		}
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		keys = append(keys, id)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 整批使用同一个已读时间。
		now := time.Now().UTC()
		changed := make([]uint, 0, len(keys))

		const size = 500

		for start := 0; start < len(keys); start += size {
			end := start + size
			if end > len(keys) {
				end = len(keys)
			}

			// RETURNING 只取出实际更新的通知 ID。
			var rows []model.Notification
			res := tx.Model(&rows).
				Clauses(clause.Returning{
					Columns: []clause.Column{{Name: "id"}},
				}).
				Where(
					"receiver_id = ? AND id IN ? AND read_at IS NULL",
					uid,
					keys[start:end],
				).
				Update("read_at", now)

			if res.Error != nil {
				return fmt.Errorf(
					"mark notifications read: %w",
					res.Error,
				)
			}

			for _, row := range rows {
				changed = append(changed, row.ID)
			}
		}

		// 没有实际变化，无需扣未读数或推进版本。
		if len(changed) == 0 {
			return nil
		}

		change := Change{
			ReadIDs: changed,
			ReadAt:  &now,
		}

		// 整批只入队一次。
		_, err := enqueueChange(
			tx,
			uid,
			change,
			//因为是已读，所以unreadcnt要减少
			-int64(len(changed)),
		)
		if err != nil {
			return fmt.Errorf(
				"enqueue notification read change: %w",
				err,
			)
		}

		return nil
	})
}
