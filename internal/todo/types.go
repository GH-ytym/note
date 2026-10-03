package todo

import (
	"time"

	"note/internal/model"
)

// Command为交给业务层的任务单

type CreateCommand struct {
	Title      string
	Content    *string
	Color      string
	StartsAt   *time.Time
	RepeatMode model.RepeatMode
	NotifyMode model.NotifyMode

	CustomDates []time.Time

	GroupID   uint
	CreatorID uint
}

type ListQuery struct {
	Page     int
	PageSize int

	GroupID uint
	UserID  uint
}

type Page struct {
	Items    []model.Todo
	Page     int
	PageSize int
	Total    int64
}

type PatchCommand struct {
	Title      *string
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
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	Color      string           `json:"color"`
	StartsAt   time.Time        `json:"starts_at"`
	OccursAt   time.Time        `json:"occurs_at"`
	RepeatMode model.RepeatMode `json:"repeat_mode"`
	NotifyMode model.NotifyMode `json:"notify_mode"`
	Version    uint             `json:"version"`

	// 当前登录用户是否完成了这一天的 Todo。
	OccurrenceDone bool `json:"occurrence_done"`
}

// 完成名单的一项，供 Handler 组装响应。
// 这是查询结果，不是数据库表。
type CompletionUser struct {
	User        model.User
	CompletedAt time.Time
}
