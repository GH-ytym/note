package model

import "time"

type User struct {
	ID uint `gorm:"primaryKey"`

	//类似瓦的命名机制，可重名，用唯一#后缀确定唯一用户
	// 相同的 Username 可以存在，但 Username + Suffix 不能重复。
	Username string `gorm:"size:80;not null;uniqueIndex:idx_users_account"`
	Suffix   int    `gorm:"not null;uniqueIndex:idx_users_account;check:users_suffix_range,suffix >= 10000 AND suffix <= 99999"`

	Email        string `gorm:"size:254;not null;uniqueIndex:idx_users_email" json:"-"`
	PasswordHash string `gorm:"not null" json:"-"`
	Nickname     string `gorm:"size:80;not null"`
	// 暂不处理头像上传；没有头像时保存空字符串。
	Avatar string `gorm:"not null;default:''" json:"avatar"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
