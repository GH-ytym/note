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

// 保存某个用户的一批通知变化。
// tx 必须是外层业务事务；先完成通知修改，再按实际变化传入 unreadDelta。
// 同一事务中同一用户的变化合并后调用一次，返回错误必须让外层事务回滚。
// 状态由注册或旧库迁移提前建立；这里不创建状态、不统计通知表。
func EnqueueChange(
	tx *gorm.DB,
	receiverID uint,
	upserts []Card, // 新增或修改后的完整 Card
	removedIDs []uint,
	unreadDelta int64, // 本次未读数变化
) (Change, error) {
	if receiverID == 0 {
		return Change{}, fmt.Errorf("receiver ID cannot be zero")
	}
	if len(upserts) == 0 && len(removedIDs) == 0 {
		return Change{}, fmt.Errorf("notification change is empty")
	}
	if unreadDelta < -mx {
		return Change{}, fmt.Errorf("unread delta is out of range")
	}

	// 已有状态只加减计数，不能用 upserts/removedIDs 的长度推算。
	// 一张 Card 可能是新增(+1)、仅修改内容(0)、或从未读变已读(-1)。
	update := tx.Model(&model.NotificationState{}).
		Where("user_id = ? AND revision >= 0 AND revision < ?", receiverID, mx).
		Where("unread_count >= 0")
	if unreadDelta >= 0 {
		update = update.Where("unread_count <= ?", mx-unreadDelta)
	} else {
		update = update.Where("unread_count >= ?", -unreadDelta)
	}
	res := update.Updates(map[string]any{
		"revision":     gorm.Expr("revision+1"),
		"unread_count": gorm.Expr("unread_count + ?", unreadDelta),
	})
	if res.Error != nil {
		return Change{}, fmt.Errorf("update notification state: %w", res.Error)
	}
	if res.RowsAffected != 1 {
		return Change{}, fmt.Errorf("notification state missing, count invalid, or revision exhausted")
	}

	//读取更新后的版本和计数
	var st model.NotificationState
	if err := tx.Where("user_id = ?", receiverID).
		First(&st).Error; err != nil {
		return Change{}, fmt.Errorf("load notification state: %w", err)
	}

	if upserts == nil {
		upserts = make([]Card, 0)
	}
	if removedIDs == nil {
		removedIDs = make([]uint, 0)
	}

	change := Change{
		Revision:    st.Revision,
		Upserts:     upserts,
		RemovedIDs:  removedIDs,
		UnreadCount: st.UnreadCount,
	}

	// 将本次变化保存到Outbox
	data, err := json.Marshal(change)
	if err != nil {
		return Change{}, fmt.Errorf("encode notification change: %w", err)
	}

	task := model.Outbox{
		ReceiverID: receiverID,
		Revision:   change.Revision,
		Name:       "notification.change",
		Data:       string(data),
		CreatedAt:  time.Now().UTC(),
	}
	if err := tx.Create(&task).Error; err != nil {
		return Change{}, fmt.Errorf("enqueue notification change: %w", err)
	}

	return change, nil

}
