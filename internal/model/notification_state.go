package model

// NotificationState 保存一个用户的通知面板状态。
// 具体通知内容仍然保存在 notifications 表中。
type NotificationState struct {
	// 一个用户只有一条状态记录，因此直接用用户 ID 作主键
	UserID uint `gorm:"primaryKey;autoIncrement:false"`

	// 用户被删除时，一并删除其通知状态
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`

	// 面板每发生一批变化，在同一个业务事务中递增一次
	Revision uint64 `gorm:"not null;default:0"`

	// 该用户全部通知中的未读总数
	UnreadCount int64 `gorm:"not null;default:0"`
}
