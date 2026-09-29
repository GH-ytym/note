package model

import "time"

type RepeatMode string

const (
	RepeatOnce     RepeatMode = "once"
	RepeatDaily    RepeatMode = "daily"
	RepeatWeekdays RepeatMode = "weekdays"
	RepeatWeekends RepeatMode = "weekends"
	RepeatWeekly   RepeatMode = "weekly"
	RepeatMonthly  RepeatMode = "monthly"
	RepeatCustom   RepeatMode = "custom"
)

type Todo struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// 标题不再全局唯一，不同 Todo 可以同名。
	Title      string     `gorm:"size:50;not null;default:''" json:"title"`
	Content    *string    `gorm:"size:500;not null" json:"content"`
	Color      string     `gorm:"size:7;not null;default:#F3B51B" json:"color"`
	StartsAt   *time.Time `gorm:"not null" json:"starts_at"`
	RepeatMode RepeatMode `gorm:"size:20;not null;default:once" json:"repeat_mode"`
	NotifyMode NotifyMode `gorm:"size:20;not null;default:none" json:"notify_mode"`
	//todocompletion不会受到alldone的影响，alldone只会影响前端状态
	AllDone   bool      `gorm:"not null;default:false" json:"all_done"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   uint      `gorm:"not null;default:1" json:"version"`

	//仅当RepeatMode为custom时才有这个
	CustomDates []TodoDate `gorm:"constraint:OnDelete:CASCADE" json:"custom_dates,omitempty"`

	// 单独授权：为本群指定成员授予 viewer 或 editor 权限。
	// 成员资格及权限判断需要由业务层执行，关联本身不提供访问控制。
	Members []TodoMember `gorm:"foreignKey:TodoID;constraint:OnDelete:CASCADE" json:"-"`
}
