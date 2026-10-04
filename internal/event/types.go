package event

import (
	"time"

	"note/internal/model"
)

// CreateCommand is the validated input passed from the HTTP layer to Service.
type CreateCommand struct {
	GroupID     uint
	CreatorID   uint
	Title       string
	Content     *string
	Color       string
	StartsAt    time.Time
	EndsAt      time.Time
	RepeatMode  model.RepeatMode
	CustomDates []time.Time
}

type ListQuery struct {
	GroupID  uint
	UserID   uint
	Page     int
	PageSize int
}

type Page struct {
	Items    []model.Event
	Page     int
	PageSize int
	Total    int64
}

type PatchCommand struct {
	Title    *string
	Content  *string
	StartsAt *time.Time
	EndsAt   *time.Time
	Version  uint
}

type CalendarOccurrence struct {
	EventID    uint             `json:"event_id"`
	GroupID    uint             `json:"group_id"`
	CreatorID  uint             `json:"creator_id"`
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	Color      string           `json:"color"`
	StartsAt   time.Time        `json:"starts_at"`
	EndsAt     time.Time        `json:"ends_at"`
	RepeatMode model.RepeatMode `json:"repeat_mode"`
	Version    uint             `json:"version"`
}
