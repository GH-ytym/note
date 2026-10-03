package model

import "time"

type TodoRole string

const (
	TodoViewer TodoRole = "viewer"
	TodoEditor TodoRole = "editor"
)

type TodoMember struct {
	// 联合主键：同一个用户在同一个 Todo 中只能有一条授权
	TodoID uint `gorm:"primaryKey;autoIncrement:false" json:"-"`
	UserID uint `gorm:"primaryKey;autoIncrement:false;index" json:"user_id"`

	// 最终权限：viewer/editor。
	// 取消编辑权限时改为 viewer。
	Role TodoRole `gorm:"size:16;not null;check:chk_todo_member_role,role IN ('viewer','editor')" json:"role"`

	User *User `gorm:"foreignKey:UserID" json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
