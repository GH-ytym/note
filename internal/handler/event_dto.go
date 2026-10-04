package handler

import (
	"time"

	"note/internal/model"
)

type CreateEventRequest struct {
	GroupID     uint             `json:"group_id" binding:"required,min=1"`
	Title       string           `json:"title" binding:"required,max=50"`
	Content     *string          `json:"content" binding:"omitempty,max=500"`
	Color       string           `json:"color" binding:"omitempty,len=7"`
	StartsAt    *time.Time       `json:"starts_at" binding:"required"`
	EndsAt      *time.Time       `json:"ends_at" binding:"required"`
	RepeatMode  model.RepeatMode `json:"repeat_mode" binding:"required,oneof=once daily weekdays weekends weekly monthly custom"`
	CustomDates []string         `json:"custom_dates"`
}

type GroupEventsURI struct {
	GroupID uint `uri:"groupID" binding:"required,min=1"`
}

type ListEventsQuery struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// 与原 Event PATCH 一样，只修改标题、内容、起止时间。
type PatchEventRequest struct {
	Title    *string    `json:"title" binding:"omitempty,max=50"`
	Content  *string    `json:"content" binding:"omitempty,max=500"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
	Version  uint       `json:"version" binding:"required,min=1"`
}

type EventDetailResponse struct {
	model.Event
	Creator     *UserSummaryResponse `json:"creator"`
	MyRole      model.EventRole      `json:"my_role"`
	MemberRoles []MemberRoleResponse `json:"member_roles"`
}

func newEventDetailResponse(item model.Event, actorIDs ...uint) EventDetailResponse {
	response := EventDetailResponse{Event: item, MyRole: model.EventViewer, MemberRoles: make([]MemberRoleResponse, 0)}
	var actorID uint
	if len(actorIDs) > 0 {
		actorID = actorIDs[0]
	}
	if item.CreatorID == actorID {
		response.MyRole = model.EventEditor
	}
	for _, member := range item.Members {
		if member.UserID == actorID && actorID != item.CreatorID {
			response.MyRole = member.Role
		}
		if actorID == item.CreatorID {
			response.MemberRoles = append(response.MemberRoles, MemberRoleResponse{UserID: member.UserID, Role: string(member.Role)})
		}
	}
	if item.Creator != nil {
		response.Creator = &UserSummaryResponse{
			ID: item.Creator.ID, Username: item.Creator.Username, Suffix: item.Creator.Suffix,
			Nickname: item.Creator.Nickname, Avatar: item.Creator.Avatar,
		}
	}
	return response
}
