package notification

import (
	"encoding/json"
	"fmt"
	"time"

	"note/internal/model"

	"gorm.io/gorm"
)

// sqlite整数上限是int64
const mx int64 = 9223372036854775807

// EnqueueChange 保存某个用户的一批普通通知变化。
// tx 必须是外层业务事务；先完成通知修改，再传入本次未读数变化。
// 返回错误时，调用方必须让外层事务回滚。
func EnqueueChange(
	tx *gorm.DB,
	receiverID uint,
	upserts []Card,
	removedIDs []uint,
	unreadDelta int64,
) (Change, error) {
	change := Change{
		Upserts:    upserts,
		RemovedIDs: removedIDs,
	}

	return enqueueChange(tx, receiverID, change, unreadDelta)
}

// enqueueChange 为一批变化分配新版本、更新未读数，并写入一条 Outbox。
// 调用前，通知记录本身应已在同一事务中修改。
// change 的 Revision 和 UnreadCount 由本函数根据数据库状态填写。
// 状态由注册或迁移提前建立；返回错误必须让外层事务回滚。
func enqueueChange(
	tx *gorm.DB,
	uid uint,
	change Change,
	delta int64,
) (Change, error) {
	if uid == 0 {
		return Change{}, fmt.Errorf("receiver ID cannot be zero")
	}

	if len(change.Upserts) == 0 &&
		len(change.RemovedIDs) == 0 &&
		len(change.ReadIDs) == 0 {
		return Change{}, fmt.Errorf("notification change is empty")
	}

	// 已读变化必须携带本批实际写入数据库的时间。
	if len(change.ReadIDs) > 0 {
		if change.ReadAt == nil || change.ReadAt.IsZero() {
			return Change{}, fmt.Errorf("read time is required")
		}
	}

	if delta < -mx {
		return Change{}, fmt.Errorf("unread delta is out of range")
	}

	// 根据实际业务变化调整计数，同时推进一次版本。
	update := tx.Model(&model.NotificationState{}).
		Where(
			"user_id = ? AND revision >= 0 AND revision < ?",
			uid,
			mx,
		).
		Where("unread_count >= 0")

	if delta >= 0 {
		update = update.Where("unread_count <= ?", mx-delta)
	} else {
		update = update.Where("unread_count >= ?", -delta)
	}

	res := update.Updates(map[string]any{
		"revision":     gorm.Expr("revision + 1"),
		"unread_count": gorm.Expr("unread_count + ?", delta),
	})
	if res.Error != nil {
		return Change{}, fmt.Errorf(
			"update notification state: %w",
			res.Error,
		)
	}
	if res.RowsAffected != 1 {
		return Change{}, fmt.Errorf(
			"notification state missing, count invalid, or revision exhausted",
		)
	}

	// 读取本次更新后的业务版本和全部未读数。
	var state model.NotificationState
	if err := tx.Where("user_id = ?", uid).
		First(&state).Error; err != nil {
		return Change{}, fmt.Errorf(
			"load notification state: %w",
			err,
		)
	}

	change.Revision = state.Revision
	change.UnreadCount = state.UnreadCount

	// 保持现有协议中的数组字段输出为 []。
	if change.Upserts == nil {
		change.Upserts = make([]Card, 0)
	}
	if change.RemovedIDs == nil {
		change.RemovedIDs = make([]uint, 0)
	}

	data, err := json.Marshal(change)
	if err != nil {
		return Change{}, fmt.Errorf(
			"encode notification change: %w",
			err,
		)
	}

	task := model.Outbox{
		ReceiverID: uid,
		Revision:   change.Revision,
		Name:       "notification.change",
		Data:       string(data),
		CreatedAt:  time.Now().UTC(),
	}
	if err := tx.Create(&task).Error; err != nil {
		return Change{}, fmt.Errorf(
			"enqueue notification change: %w",
			err,
		)
	}

	return change, nil
}
