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
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Version    uint       `gorm:"not null;default:1" json:"version"`

	//仅当RepeatMode为custom时才有这个
	CustomDates []TodoDate `gorm:"constraint:OnDelete:CASCADE" json:"custom_dates,omitempty"`

	// 本群成员对这条 Todo 的最终权限，每人一条记录。
	Members []TodoMember `gorm:"foreignKey:TodoID;constraint:OnDelete:CASCADE" json:"-"`

	GroupID uint   `gorm:"not null;index" json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID;constraint:OnDelete:RESTRICT" json:"-"`

	CreatorID uint  `gorm:"not null;index" json:"creator_id"`
	Creator   *User `gorm:"foreignKey:CreatorID;constraint:OnDelete:RESTRICT" json:"-"`
}
