package handler

import (
	"note/internal/model"
	"time"
)

type CreateGroupRequest struct {
	Name string `json:"name" binding:"required,max=80"`
}

type GroupResponse struct {
	Code      string            `json:"code"`
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

// 加入群组的request
type JoinGroupRequest struct {
	// 固定公开群号，不是可刷新的邀请码；无需提供内部 GroupID。
	Code string `json:"code" binding:"required"`
}

// 群主退群的时候要先转让群主
type QuitGroupRequest struct {
	//nil表示不转让（普通用户就是nil）
	Target *uint `json:"target" binding:"omitempty,min=1"`
}

// URL 使用 Card.JoinRequestID，也就是申请表的主键；不是 Card.ID。
// 后端仍需查询申请最新状态，并验证审核人的权限。
type ReviewGroupJoinURI struct {
	JoinRequestID uint `uri:"joinRequestID" binding:"required,min=1"`
}

// 只表达审核决定；审核人从 JWT 获取，群组和申请人从数据库查询。
type ReviewGroupJoinRequest struct {
	Decision string `json:"decision" binding:"required,oneof=accept reject"`
}
