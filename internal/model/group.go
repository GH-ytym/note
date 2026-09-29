package model

import "time"

type Group struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 不同群组允许重名，使用 ID 区分。
	Name string `gorm:"size:80;not null" json:"name"`

	// 群主：负责管理这个群组。
	OwnerID uint  `gorm:"not null;index" json:"owner_id"`
	Owner   *User `gorm:"foreignKey:OwnerID;constraint:OnDelete:RESTRICT" json:"-"`

	// 一个群组对应多条成员关系。
	Members []GroupMember `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
