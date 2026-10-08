package notification

import (
	"fmt"
	"time"

	"note/internal/model"
)

// Card 是一条通知的展示数据，作为 Inbox.Items 中的一项返回前端。
type Card struct {
	// Notification.ID：前端用来标识通知；不是申请 ID 或 Redis Stream ID。
	// 后续审批可由后端根据这个通知 ID 查询关联的 JoinRequestID。
	ID uint `json:"id"`
	// 通知内容类别：joined 表示已经入群，join_requested 表示入群申请。
	// 与 Event.Name（通知创建／更新）和 RequestStatus（申请处理进度）不同。
	Type model.NotificationType `json:"type"`

	// 触发这条通知的用户，不是通知接收者。
	ActorID     uint   `json:"actor_id"`
	ActorName   string `json:"actor_name"`
	ActorAvatar string `json:"actor_avatar"`

	// 这条通知涉及的群组，不是申请记录的 ID。
	GroupID   uint   `json:"group_id"`
	GroupName string `json:"group_name"`

	// 关联申请／邀请的处理状态，例如 pending、accepted、rejected、cancelled。
	// public 直接入群没有申请，此字段为空，omitempty 会省略它。
	// 已读状态单独看 ReadAt，读过通知不代表已经处理申请。
	RequestStatus model.Status `json:"request_status,omitempty"`

	// nil 表示通知未读，与 RequestStatus 相互独立。
	ReadAt *time.Time `json:"read_at"`
	// 通知生成时间；申请过期仍按数据库 GroupJoinRequest.CreatedAt 判断。
	CreatedAt time.Time `json:"created_at"`
}

// Inbox 是列表接口返回的完整响应，包含通知卡片和全部未读数量。
type Inbox struct {
	Items       []Card `json:"items"`
	UnreadCount int64  `json:"unread_count"`
}

func NewCard(n model.Notification) (Card, error) {
	if n.Actor == nil || n.Group == nil {
		return Card{}, fmt.Errorf(
			"notification %d: actor or group not loaded", n.ID,
		)
	}

	if n.JoinRequestID != nil && n.JoinRequest == nil {
		return Card{}, fmt.Errorf(
			"notification %d: join request not loaded", n.ID,
		)
	}

	card := Card{
		ID:          n.ID,
		Type:        n.Type,
		ActorID:     n.ActorID,
		ActorName:   n.Actor.Nickname,
		ActorAvatar: n.Actor.Avatar,
		GroupID:     n.GroupID,
		GroupName:   n.Group.Name,
		ReadAt:      n.ReadAt,
		CreatedAt:   n.CreatedAt,
	}

	if card.ActorName == "" {
		card.ActorName = fmt.Sprintf(
			"%s#%05d", n.Actor.Username, n.Actor.Suffix,
		)
	}

	if req := n.JoinRequest; req != nil {
		card.RequestStatus = req.Status
	}

	return card, nil
}
