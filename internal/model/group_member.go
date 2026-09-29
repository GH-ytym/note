package model

import "time"

// 暂时只记录成员资格，组内角色和权限放到第二步设计。
type GroupMember struct {
	GroupID uint `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	UserID  uint `gorm:"primaryKey;autoIncrement:false;index" json:"user_id"`

	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-"`

	JoinedAt time.Time `gorm:"autoCreateTime" json:"joined_at"`
}
