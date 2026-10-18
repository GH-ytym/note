package model

import "time"

// Outbox 保存等待后台投递的通知事件。
type Outbox struct {
	// 投递任务 ID，供后台发布时去重。
	ID uint `gorm:"primaryKey;autoIncrement"`

	// 这条事件发给哪个用户。
	ReceiverID uint `gorm:"not null"`

	// 为后续版本机制保留。
	// 当前旧链路暂时不分配版本，统一为 0。
	Revision uint64 `gorm:"not null;default:0"`

	// 事件名称，例如 notification.created。
	Name string `gorm:"not null"`

	// 当前保存本次通知 Card 的 JSON。
	Data string `gorm:"type:text;not null"`

	CreatedAt time.Time `gorm:"not null"`

	// nil 表示尚未成功交给 Redis。
	// 有值只代表发布成功，不代表用户已经收到或读过。
	PublishedAt *time.Time `gorm:"index"`
}
