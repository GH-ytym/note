package handler

import (
	"note/internal/model"
	"time"
)

type CreateGroupRequest struct {
	Name string `json:"name" binding:"required,max=80"`
}

type GroupResponse struct {
	ID        uint              `json:"id"`
	Name      string            `json:"name"`
	OwnerID   uint              `json:"owner_id"`
	Policy    model.GroupPolicy `json:"policy"`
	CreatedAt time.Time         `json:"created_at"`
}

// 成员列表复用公开用户资料，不返回邮箱或密码等账号信息。
type GroupMemberResponse struct {
	UserSummaryResponse
	JoinedAt time.Time `json:"joined_at"`
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

// 群主退群的时候要先转让群主
type QuitGroupRequest struct {
	//nil表示不转让（普通用户就是nil）
	Target *uint `json:"target" binding:"omitempty,min=1"`
}
