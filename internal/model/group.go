package model

import "time"

type GroupPolicy string

const (
	Restricted GroupPolicy = "restricted" //只有群主可以邀请入群；接收者同意即入群；新成员不可以根据验证码申请加群
	Public     GroupPolicy = "public"     //群成员均可邀请新成员入群，接收者同意即入群；新成员可以以根据验证码直接加群，群主无需审核
	Approval   GroupPolicy = "approval"   //群成员均可邀请新成员入群，接收者同意即入群；新成员可以根据验证码申请加群，群主同意后入群
	Personal   GroupPolicy = "personal"   //单人空间，无法邀请或申请加群，无法解散、退出或删除
)

func (p GroupPolicy) Valid() bool {
	switch p {
	case Restricted, Public, Approval, Personal:
		return true
	default:
		return false
	}
}

type Group struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 不同群组允许重名，使用 ID 区分。
	Name string `gorm:"size:80;not null" json:"name"`

	// 邀请和申请的规则，默认为public
	Policy GroupPolicy `gorm:"not null;default:public;check:ck_groups_policy,policy IN ('restricted','public','approval','personal')" json:"policy"`

	// 群主：负责管理这个群组。
	OwnerID uint  `gorm:"not null;index" json:"owner_id"`
	Owner   *User `gorm:"foreignKey:OwnerID;constraint:OnDelete:RESTRICT" json:"-"`

	// 一个群组对应多条成员关系。
	Members []GroupMember `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 邀请通过 GroupID + Code 验证，不同群可以使用相同的邀请码。
	Code string `gorm:"size:6;not null" json:"-"`
}
