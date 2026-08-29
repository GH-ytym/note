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
	ID         uint       `gorm:"primaryKey" json:"id"`
	Content    string     `gorm:"size:500;not null;uniqueIndex" json:"content"`
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
}
