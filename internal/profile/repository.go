package profile

import (
	"context"
	"database/sql"
	"note/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	Get(context.Context, uint) (model.User, error)
	SetAvatar(context.Context, uint, string) (model.User, string, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db} }

func (r *repository) Get(ctx context.Context, id uint) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Select("id", "username", "suffix", "nickname", "avatar").First(&user, id).Error
	return user, err
}

// 在事务中取旧地址并更新，避免两次并发上传清理错对象。
func (r *repository) SetAvatar(ctx context.Context, id uint, avatar string) (model.User, string, error) {
	var user model.User
	var old string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id", "username", "suffix", "nickname", "avatar").First(&user, id).Error; err != nil {
			return err
		}
		old = user.Avatar
		if err := tx.Model(&model.User{}).Where("id = ?", id).Update("avatar", avatar).Error; err != nil {
			return err
		}
		user.Avatar = avatar
		return nil
	}, &sql.TxOptions{})
	return user, old, err
}
