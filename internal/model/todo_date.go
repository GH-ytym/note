package model

import "time"

// TodoDate 保存 custom 模式下用户选择的一个日期。
// 同一个 Todo 的同一天只能保存一次。
type TodoDate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TodoID    uint      `gorm:"not null;uniqueIndex:idx_todo_date" json:"todo_id"`
	Date      time.Time `gorm:"type:date;not null;uniqueIndex:idx_todo_date" json:"date"`
	CreatedAt time.Time `json:"created_at"`
}
