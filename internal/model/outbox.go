package model

import "time"

// 记录后台需要投递的事件。
type Outbox struct {
	// 投递任务 ID，用于发布去重；不是 Notification.ID 或 Redis Stream ID。
	ID uint `gorm:"primaryKey;autoIncrement"`

	// 事件发给哪个用户。
	ReceiverID uint `gorm:"not null"`

	// 推送动作名，例如 notification.created；与 Card.Type 的通知内容类别不同。
	Name string `gorm:"not null"`

	// 本次要发送的 Card JSON 快照，与 Event.Data 对应。
	Data string `gorm:"type:text;not null"`

	CreatedAt time.Time `gorm:"not null"`

	// nil：尚未成功交给 Redis。
	// 有值：已成功交给 Redis，不代表用户已经收到或读过。
	PublishedAt *time.Time `gorm:"index"`
}
