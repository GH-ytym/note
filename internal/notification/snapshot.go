package notification

import (
	"encoding/json"
	"fmt"
	"note/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 在同一事务中保存投递任务
// snapshot代表了某次通知面板长什么样，它依旧只算一次任务，只是可能包含多个card
func EnqueueInboxSnapshot(tx *gorm.DB, receiverID uint) error {
	//第一次使用先创建快照记录
	state := model.NotificationState{
		UserID: receiverID,
	}
	if err := tx.
		Omit("User").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).
		Create(&state).Error; err != nil {
		return fmt.Errorf("initialize notification state: %w", err)
	}

	//先递增revision
	if err := tx.Model(&model.NotificationState{}).
		Where("user_id = ?", receiverID).
		UpdateColumn("revision", gorm.Expr("revision + 1")).
		Error; err != nil {
		return fmt.Errorf("increment notification revision: %w", err)
	}

	//再读取新版本
	err := tx.Where("receiver_id = ?", receiverID).First(&state).Error
	if err != nil {
		return fmt.Errorf("get notification state: %w", err)
	}
	inbox := Inbox{
		Revision: state.Revision,
		Items:    make([]Card, 0),
	}

	//完整通知面板
	//这里是统计全部的未读数量
	if err := tx.Model(model.Notification{}).
		Where("receiver_id = ? and read_at is NULL", receiverID).
		Count(&inbox.UnreadCount).Error; err != nil {
		return fmt.Errorf("get unread count: %w", err)
	}

	//批量拿出未读通知
	var items []model.Notification

	// 复用 repository.go 中已有的关联加载函数。
	if err := preloadCardRelations(
		tx.Where("receiver_id = ?", receiverID),
	).
		Order("id DESC").
		Limit(50).
		Find(&items).Error; err != nil {
		return fmt.Errorf("list notification snapshot: %w", err)
	}
	for _, item := range items {
		card, err := NewCard(item)
		if err != nil {
			return fmt.Errorf("new card: %w", err)
		}
		inbox.Items = append(inbox.Items, card)
	}

	//编码整个json
	data, err := json.Marshal(inbox)
	if err != nil {
		return fmt.Errorf("encode inbox: %w", err)
	}

	//保存投递任务
	//丢进outbox
	task := model.Outbox{
		ReceiverID: receiverID,
		Revision:   inbox.Revision,
		Name:       "notification.snapshot",
		Data:       string(data),
		CreatedAt:  time.Now().UTC(),
	} //现在一次task有一整批card了

	if err := tx.Create(&task).Error; err != nil {
		return fmt.Errorf("enqueue notification snapshot: %w", err)
	}
	return nil
}
