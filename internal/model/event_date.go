package model

import "time"

// EventDate stores one selected date for a custom recurring event.
type EventDate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventID   uint      `gorm:"not null;uniqueIndex:idx_event_date" json:"event_id"`
	Date      time.Time `gorm:"type:date;not null;uniqueIndex:idx_event_date" json:"date"`
	CreatedAt time.Time `json:"created_at"`
}
