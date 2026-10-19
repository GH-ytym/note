package notification

import (
	"fmt"
	"time"

	"note/internal/model"
)

// Card 是一条通知的展示数据，作为 Inbox.Items 中的一项返回前端。
type Card struct {
	// Notification.ID：前端用来标识通知；不是申请 ID 或 Redis Stream ID。
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

	// 关联申请／邀请的业务 ID，审核时使用；不同于本卡片的通知 ID。
	// 普通入群提醒没有关联申请，nil 时不返回该字段。
	JoinRequestID *uint `json:"join_request_id,omitempty"`

	// 关联申请/邀请的处理状态，例如 pending、accepted、rejected、cancelled。
	// public 直接入群没有申请，此字段为空，omitempty 会省略它。
	// 已读状态单独看 ReadAt
	RequestStatus model.Status `json:"request_status,omitempty"`

	// nil 表示通知未读，与 RequestStatus 相互独立。
	ReadAt *time.Time `json:"read_at"`
	// 通知生成时间；申请过期仍按数据库 GroupJoinRequest.CreatedAt 判断。
	CreatedAt time.Time `json:"created_at"`
}

// Inbox 是通知面板的完整快照。
// 用于初始加载或恢复时重置面板；正常增量变化使用 Change。
type Inbox struct {
	// JSON 中输出为字符串，避免 JavaScript 大整数精度丢失
	Revision uint64 `json:"revision,string"`

	// 按通知 ID 倒序排列；缓存初始化加载全部，列表接口的返回范围由查询决定。
	Items []Card `json:"items"`

	// 全部未读通知数量
	UnreadCount int64 `json:"unread_count"`
}

// Change 表示某个用户的通知面板发生的一批变化。
// 同一批可以涉及多张通知，但只对应一个新版本。
type Change struct {
	// 应用这批变化之后，面板应该到达的版本。
	Revision uint64 `json:"revision,string"`

	// 新增或更新的通知，携带每张 Card 的完整内容。
	// 同 ID 已存在则替换，不存在则作为新增候选。
	Upserts []Card `json:"upserts"`

	// 需要移除的通知 ID，使用 Notification.ID。
	RemovedIDs []uint `json:"removed_ids"`

	// 变化之后的全部未读总数
	UnreadCount int64 `json:"unread_count"`
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
		ID:            n.ID,
		Type:          n.Type,
		ActorID:       n.ActorID,
		ActorName:     n.Actor.Nickname,
		ActorAvatar:   n.Actor.Avatar,
		GroupID:       n.GroupID,
		GroupName:     n.Group.Name,
		JoinRequestID: n.JoinRequestID,
		ReadAt:        n.ReadAt,
		CreatedAt:     n.CreatedAt,
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
