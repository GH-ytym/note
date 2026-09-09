package model

import "time"

// Event occupies a precise time range on the calendar.
// It is intentionally separate from Todo because an event has duration but
// does not have a completion state.
type Event struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	Title       string      `gorm:"size:50;not null" json:"title"`
	Content     *string     `gorm:"size:500" json:"content"`
	Color       string      `gorm:"size:7;not null;default:#F3B51B" json:"color"`
	StartsAt    time.Time   `gorm:"not null;index" json:"starts_at"`
	EndsAt      time.Time   `gorm:"not null;index;check:chk_events_time_range,ends_at > starts_at" json:"ends_at"`
	RepeatMode  RepeatMode  `gorm:"size:20;not null;default:once" json:"repeat_mode"`
	CustomDates []EventDate `gorm:"constraint:OnDelete:CASCADE" json:"custom_dates,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Version     uint        `gorm:"not null;default:1" json:"version"`
}
