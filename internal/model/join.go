package model

import "time"

// 申请／邀请业务记录的类型，保存在 GroupJoinRequest.Kind。
// Card 用通知的 Type 区分展示内容，不再重复返回 Kind。
type Kind string

// 业务状态：待处理 接受 拒绝 取消；已读状态由通知记录保存
type Status string

const (
	Invitation  Kind = "invitation"
	Application Kind = "application"

	Pending   Status = "pending"
	Accepted  Status = "accepted"
	Rejected  Status = "rejected"
	Cancelled Status = "cancelled"
)

// GroupJoinRequest 保存申请／邀请本身，通知通过 JoinRequestID 关联它。
type GroupJoinRequest struct {
	// 申请／邀请的业务 ID；不同于 Notification.ID 和 Redis Stream ID。
	ID uint `gorm:"primaryKey" json:"id"`

	GroupID uint   `gorm:"not null;index" json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`

	Kind Kind `gorm:"not null;check:ck_kind,kind IN ('invitation','application')" json:"kind"`

	// application 时是申请人；invitation 时是邀请者。
	SenderID uint  `gorm:"not null;index" json:"sender_id"`
	Sender   *User `gorm:"foreignKey:SenderID;constraint:OnDelete:RESTRICT" json:"-"`

	// application 时是接收申请的群主；invitation 时是被邀请者。
	ReceiverID uint  `gorm:"not null;index" json:"receiver_id"`
	Receiver   *User `gorm:"foreignKey:ReceiverID;constraint:OnDelete:RESTRICT" json:"-"`

	// 业务处理进度；读取通知不会改变它，Card.RequestStatus 来自此字段。
	Status Status `gorm:"not null;default:pending;check:ck_status,status IN ('pending','accepted','rejected','cancelled')" json:"status"`

	// 待处理时为 nil；接受、拒绝或撤销时填入。
	HandledAt *time.Time `json:"handled_at"`

	// 申请／邀请的七天有效期从这里计算，不依赖通知的 CreatedAt 或已读时间。
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
