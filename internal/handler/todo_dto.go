package handler

import (
	"time"

	"note/internal/model"
)

//DTO为http请求表单

// DTO of Creating a Todo
type CreateTodoRequest struct {
	Title      string           `json:"title" binding:"required,max=50"`
	Content    *string          `json:"content" binding:"omitempty,max=500"`
	Color      string           `json:"color" binding:"omitempty,len=7"`
	StartsAt   *time.Time       `json:"starts_at" binding:"required"`
	RepeatMode model.RepeatMode `json:"repeat_mode" binding:"required,oneof=once daily weekdays weekends weekly monthly custom"`
	NotifyMode model.NotifyMode `json:"notify_mode" binding:"omitempty,oneof=none silent popup"`

	//当需要自定义日期时用这个（创建日程肯定不能把日期留空）
	CustomDates []string `json:"custom_dates"`
}

// DTO of Query
type ListTodosQuery struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// DTO of patch
type PatchTodoRequest struct {
	Title      *string           `json:"title" binding:"omitempty,max=50"`
	Content    *string           `json:"content" binding:"omitempty,max=500"`
	Color      *string           `json:"color" binding:"omitempty,len=7"`
	StartsAt   *time.Time        `json:"starts_at"`
	RepeatMode *model.RepeatMode `json:"repeat_mode" binding:"omitempty,oneof=once daily weekdays weekends weekly monthly custom"`
	NotifyMode *model.NotifyMode `json:"notify_mode" binding:"omitempty,oneof=none silent popup"`
	AllDone    *bool             `json:"all_done"`
	Version    uint              `json:"version" binding:"required,min=1"`
	// nil 表示请求没传；指向空切片表示用户明确清空日期。
	CustomDates *[]string `json:"custom_dates"`
}

// DTO of calender query
type CalendarQuery struct {
	From string `form:"from" binding:"required"`
	To   string `form:"to" binding:"required"`
}

// DTO of single occurence
type PatchOccurrenceRequest struct {
	Done *bool `json:"done" binding:"required"`
}
