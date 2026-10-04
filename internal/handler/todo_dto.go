package handler

import (
	"time"

	"note/internal/model"
)

// UserSummaryResponse 只返回用于展示的用户资料。
type UserSummaryResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Suffix   int    `json:"suffix"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// 保留原来 Todo 的 JSON 字段，额外返回创建者资料。
// model.Todo.Creator 仍为 json:"-"，通过这里明确选择要返回的字段。
type TodoDetailResponse struct {
	model.Todo
	Creator *UserSummaryResponse `json:"creator"`
}

func newTodoDetailResponse(item model.Todo) TodoDetailResponse {
	response := TodoDetailResponse{Todo: item}
	if item.Creator != nil {
		response.Creator = &UserSummaryResponse{
			ID:       item.Creator.ID,
			Username: item.Creator.Username,
			Suffix:   item.Creator.Suffix,
			Nickname: item.Creator.Nickname,
			Avatar:   item.Creator.Avatar,
		}
	}
	return response
}

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

	GroupID uint `json:"group_id" binding:"required,min=1"`
}

// 接收 /groups/:groupID/todos 中的路径参数。
type GroupTodosURI struct {
	GroupID uint `uri:"groupID" binding:"required,min=1"`
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

// 嵌入用户展示资料，再增加这个用户的完成时间
type CompletedUserResponse struct {
	UserSummaryResponse
	CompletedAt time.Time `json:"completed_at"`
}

// 某一个todo某一天的完成名单
type OccurrenceCompletionsResponse struct {
	TodoID         uint                    `json:"todo_id"`
	CompletedCount int                     `json:"completed_count"`
	Users          []CompletedUserResponse `json:"users"`
}

// 改权限的dto
type RolesRequest struct {
	// 本次需要修改的用户。
	UserIDs []uint `json:"user_ids" binding:"required,min=1,max=100,dive,min=1"`

	// 1：editor  2：viewer
	//不用管之前是什么权限
	Role uint `json:"role" binding:"required,oneof=1 2"`
}
