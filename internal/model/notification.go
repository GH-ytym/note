package model

import "time"

// 通知内容类别：joined 是已经入群的提醒，join_requested 是入群申请通知。
// 邀请／申请的处理进度保存在 GroupJoinRequest.Status；推送动作看 notification.Event.Name。
type NotificationType string

const (
	Joined        NotificationType = "joined"
	JoinRequested NotificationType = "join_requested"
)

// 保存在数据库中的通知记录，记录接收者、触发者和关联群组／申请。
type Notification struct {
	// 通知记录的 ID；Card.ID 使用它，申请记录和 Redis 事件各有自己的 ID。
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

	// 指向这条通知对应的申请／邀请记录；后端可从通知 ID 查到这个申请 ID。
	// public 直接入群没有申请，所以允许为 nil；Card 不再返回申请 ID。
	JoinRequestID *uint             `gorm:"index" json:"join_request_id"`
	JoinRequest   *GroupJoinRequest `gorm:"foreignKey:JoinRequestID;constraint:OnDelete:SET NULL" json:"-"`

	Type NotificationType `gorm:"not null;check:ck_notifications_type,type IN ('joined','join_requested')" json:"type"`

	// nil 表示通知未读；与 GroupJoinRequest.Status 是否已处理相互独立。
	ReadAt *time.Time `gorm:"index:idx_notifications_receiver_read,priority:2" json:"read_at"`
	// 通知生成时间；申请／邀请的七天有效期从 GroupJoinRequest.CreatedAt 计算。
	CreatedAt time.Time `json:"created_at"`
}
