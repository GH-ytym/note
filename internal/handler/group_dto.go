package handler

import "time"

type CreateGroupRequest struct {
	Name string `json:"name" binding:"required,max=80"`
}

type GroupResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	OwnerID   uint      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// URL 中的群组 ID
type GroupURI struct {
	GroupID uint `uri:"groupID" binding:"required,min=1"`
}

// 专门返回邀请信息，普通群资料响应不包含邀请码
type GroupInviteResponse struct {
	GroupID uint   `json:"group_id"`
	Code    string `json:"code"`
}

// 加入群组的request
type JoinGroupRequest struct {
	Code string `json:"code" binding:"required"`
}
