package model

import "time"

// 通知事件的种类；邀请／申请的处理状态保存在 GroupJoinRequest.Status。
type NotificationType string

const (
	Joined        NotificationType = "joined"
	JoinRequested NotificationType = "join_requested"
)

// 保存在数据库中的通知记录，记录接收者、触发者和关联群组／申请。
type Notification struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 通知发给谁
	ReceiverID uint  `gorm:"not null;index:idx_notifications_receiver_read,priority:1" json:"receiver_id"`
	Receiver   *User `gorm:"foreignKey:ReceiverID;constraint:OnDelete:CASCADE" json:"-"`

	// 谁触发了这条通知，这里就是申请或加入群组的人
	ActorID uint  `gorm:"not null" json:"actor_id"`
	Actor   *User `gorm:"foreignKey:ActorID;constraint:OnDelete:RESTRICT" json:"-"`

	//发送的群组
	GroupID uint   `gorm:"not null;index" json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`

	// public 入群没有申请，所以允许为 nil
	JoinRequestID *uint             `gorm:"index" json:"join_request_id"`
	JoinRequest   *GroupJoinRequest `gorm:"foreignKey:JoinRequestID;constraint:OnDelete:SET NULL" json:"-"`

	Type NotificationType `gorm:"not null;check:ck_notifications_type,type IN ('joined','join_requested')" json:"type"`

	// nil 表示未读；读过后保存读取时间
	ReadAt    *time.Time `gorm:"index:idx_notifications_receiver_read,priority:2" json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}
