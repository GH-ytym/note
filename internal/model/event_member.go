package model

import "time"

type EventRole string

const (
	EventViewer EventRole = "viewer"
	EventEditor EventRole = "editor"
)

// EventMember 只保存编辑权限，不表示报名、签到或完成状态。
// 群内成员默认全部可见、全部参与。
type EventMember struct {
	EventID   uint      `gorm:"primaryKey;autoIncrement:false" json:"-"`
	UserID    uint      `gorm:"primaryKey;autoIncrement:false;index" json:"user_id"`
	Role      EventRole `gorm:"size:16;not null;check:chk_event_member_role,role IN ('viewer','editor')" json:"role"`
	User      *User     `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
