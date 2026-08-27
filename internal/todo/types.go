package todo

import (
	"time"

	"note/internal/model"
)

// Command为交给业务层的任务单

type CreateCommand struct {
	Content    string
	Color      string
	StartsAt   *time.Time
	RepeatMode model.RepeatMode
	NotifyMode model.NotifyMode

	CustomDates []time.Time
}

type ListQuery struct {
	Page     int
	PageSize int
}

type Page struct {
	Items    []model.Todo
	Page     int
	PageSize int
	Total    int64
}

type PatchCommand struct {
	Content    *string
	Color      *string
	StartsAt   *time.Time
	RepeatMode *model.RepeatMode
	NotifyMode *model.NotifyMode
	Version    uint

	CustomDates *[]time.Time
}

type CalendarOccurrence struct {
	TodoID     uint             `json:"todo_id"`
	Content    string           `json:"content"`
	Color      string           `json:"color"`
	Done       bool             `json:"done"`
	StartsAt   time.Time        `json:"starts_at"`
	OccursAt   time.Time        `json:"occurs_at"`
	RepeatMode model.RepeatMode `json:"repeat_mode"`
	NotifyMode model.NotifyMode `json:"notify_mode,omitempty"`
	Version    uint             `json:"version"`
}
