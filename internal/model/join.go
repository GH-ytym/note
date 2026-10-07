package model

import "time"

// 卡片类型：邀请或者申请
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
//保存申请本身
type GroupJoinRequest struct {
	ID uint `gorm:"primaryKey" json:"id"`

	GroupID uint   `gorm:"not null;index" json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`

	Kind Kind `gorm:"not null;check:ck_kind,kind IN ('invitation','application')" json:"kind"`

	SenderID uint  `gorm:"not null;index" json:"sender_id"`
	Sender   *User `gorm:"foreignKey:SenderID;constraint:OnDelete:RESTRICT" json:"-"`

	ReceiverID uint  `gorm:"not null;index" json:"receiver_id"`
	Receiver   *User `gorm:"foreignKey:ReceiverID;constraint:OnDelete:RESTRICT" json:"-"`

	Status Status `gorm:"not null;default:pending;check:ck_status,status IN ('pending','accepted','rejected','cancelled')" json:"status"`

	// 待处理时为 nil；接受、拒绝或撤销时填入。
	HandledAt *time.Time `json:"handled_at"`

	// 邀请/申请都是七天过期
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
