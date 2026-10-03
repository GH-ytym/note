package model

import "time"

// JSON 数组中的一个元素，不是新表。
type CompletionEntry struct {
	UserID      uint      `json:"user_id"`
	CompletedAt time.Time `json:"completed_at"`
}

// 一条记录表示某条 Todo 的某一天。
// Records 保存这一天所有完成用户及其完成时间。
type TodoCompletion struct {
	ID uint `gorm:"primaryKey" json:"id"`

	TodoID uint `gorm:"not null;uniqueIndex:idx_todo_completion" json:"todo_id"`

	OccursOn time.Time `gorm:"type:date;not null;uniqueIndex:idx_todo_completion" json:"occurs_on"`

	Records []CompletionEntry `gorm:"serializer:json;type:text" json:"records"`

	Todo Todo `gorm:"foreignKey:TodoID;constraint:OnDelete:CASCADE" json:"-"`
}
