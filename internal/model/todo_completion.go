package model

import "time"

// TodoCompletion 表示某个 Todo 在某一天已经完成。
// 没有对应记录时，该次日程默认视为未完成。
// 只记录完成而不记录出现，可以降低表的膨胀速度
type TodoCompletion struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TodoID      uint      `gorm:"not null;uniqueIndex:idx_todo_completion" json:"todo_id"`
	OccursOn    time.Time `gorm:"type:date;not null;uniqueIndex:idx_todo_completion" json:"occurs_on"`
	CompletedAt time.Time `gorm:"not null" json:"completed_at"`

	Todo Todo `gorm:"foreignKey:TodoID;constraint:OnDelete:CASCADE" json:"-"`
}
