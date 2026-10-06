package model

import "time"

// 记录后台需要投递的事件。
type Outbox struct {
	// 这次投递任务的 ID，后续用于发布去重。
	ID uint `gorm:"primaryKey;autoIncrement"`

	// 事件发给哪个用户。
	ReceiverID uint `gorm:"not null"`

	// 推送事件名，例如 notification.created。
	Name string `gorm:"not null"`

	// 本次要发送的 Card JSON 快照，与 Event.Data 对应。
	Data string `gorm:"type:text;not null"`

	CreatedAt time.Time `gorm:"not null"`

	// nil：尚未成功交给 Redis。
	// 有值：已成功交给 Redis，不代表用户已经收到或读过。
	PublishedAt *time.Time `gorm:"index"`
}
