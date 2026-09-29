package auth

import (
	"context"
	"errors"

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
	// 只忽略完整账号的冲突，不更新已有用户。
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "username"}, {Name: "suffix"}},
		DoNothing: true,
	}).Create(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			// 确认邮箱冲突，避免将主键等其他唯一冲突误报成邮箱重复。
			_, err := r.FindByEmail(ctx, user.Email)
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
	return nil
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
