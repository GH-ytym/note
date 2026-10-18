package auth

import (
	"context"
	"errors"
	"fmt"

	apperrors "note/internal/errors"
	"note/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Create(ctx context.Context, user *model.User) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByAccount(
		ctx context.Context,
		username string,
		suffix int,
	) (*model.User, error)
	FindByID(ctx context.Context, userID uint) (*model.User, error) //redis要用这个
}

type gormRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(
	ctx context.Context,
	user *model.User,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 用户和初始通知状态一起创建，任意一步失败都回滚。
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}, {Name: "suffix"}},
			DoNothing: true,
		}).Create(user)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
				// 确认邮箱冲突，避免将主键等其他唯一冲突误报成邮箱重复。
				// 使用当前事务，避免单连接数据库等待自己释放连接。
				var existing model.User
				err := tx.Where("email = ?", user.Email).First(&existing).Error
				if err == nil {
					return apperrors.ErrEmailTaken
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return result.Error
		}
		//RowsAffected == 0说明重复了
		if result.RowsAffected == 0 {
			return apperrors.ErrAccountTaken
		}

		//通知状态初始化
		//这样就可以借着初始化的0来实现增量更新，免得每次都查数据库才能拿到unreadcnt
		state := model.NotificationState{UserID: user.ID}
		if err := tx.Omit("User").Create(&state).Error; err != nil {
			return fmt.Errorf("initialize user notification state: %w", err)
		}
		return nil
	})
}

func (r *gormRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).
		Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *gormRepository) FindByAccount(
	ctx context.Context,
	username string,
	suffix int,
) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).
		Where("username = ? AND suffix = ?", username, suffix).
		First(&user).
		Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *gormRepository) FindByID(ctx context.Context, userID uint) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).
		First(&user, userID).
		Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}
