package notification

import (
	"fmt"
	"time"

	"note/internal/model"
)

// RequestCard 是邀请／申请的展示资料，作为 Card.Request 嵌套返回前端。
type RequestCard struct {
	ID         uint         `json:"id"`
	Kind       model.Kind   `json:"kind"`
	SenderID   uint         `json:"sender_id"`
	ReceiverID uint         `json:"receiver_id"`
	Status     model.Status `json:"status"`

	CreatedAt time.Time  `json:"created_at"`
	HandledAt *time.Time `json:"handled_at"`
}

// Card 是一条通知的展示数据，作为 Inbox.Items 中的一项返回前端。
type Card struct {
	ID   uint                   `json:"id"`
	Type model.NotificationType `json:"type"`

	ActorID     uint   `json:"actor_id"`
	ActorName   string `json:"actor_name"`
	ActorAvatar string `json:"actor_avatar"`

	GroupID   uint   `json:"group_id"`
	GroupName string `json:"group_name"`

	// public 入群没有申请，这里可以为 nil。
	Request *RequestCard `json:"request"`

	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
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
		card.Request = &RequestCard{
			ID:         req.ID,
			Kind:       req.Kind,
			SenderID:   req.SenderID,
			ReceiverID: req.ReceiverID,
			Status:     req.Status,
			CreatedAt:  req.CreatedAt,
			HandledAt:  req.HandledAt,
		}
	}

	return card, nil
}
