package model

import "time"

//不要把请求绑定到user
//单独定义请求体，避免用户提交不该由用户控制的字段
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"size:32;not null;uniqueIndex"`
	PasswordHash string `gorm:"not null" json:"-"`
	Nickname     string `gorm:"size:50;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
