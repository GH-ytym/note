package group

import (
	"context"
	"errors"
	"fmt"
	apperrors "note/internal/errors"
	"note/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, item *model.Group) error
	ListByUserID(
		ctx context.Context,
		userID uint,
	) ([]model.Group, error)
}

type gormRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(
	ctx context.Context,
	item *model.Group,
) error {
	// WithContext 让数据库操作受本次请求的取消/超时控制；它本身不开启事务。
	return r.db.WithContext(ctx).
		Transaction(
			func(tx *gorm.DB) error {
				var owner model.User
				// JWT 有效也可能对应已删除的账号，因此确认用户仍存在。
				// 这里只需要确认 ID，不必查询密码、邮箱等其他字段。
				err := tx.Select("id").First(&owner, item.OwnerID).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return apperrors.ErrGroupUnauthenticated
				}
				if err != nil {
					return fmt.Errorf("find group owner: %w", err)
				}
				// 先插入群，再填入成员的 GroupID
				if err := tx.
					Omit("Owner", "Members.User"). //跳过owner和members.User
					Create(item).
					Error; err != nil {
					return fmt.Errorf("create group with owner membership: %w", err)
				}
				return nil
			},
		)
}

func (r *gormRepository) ListByUserID(
	ctx context.Context,
	userID uint,
) ([]model.Group, error) {
	items := make([]model.Group, 0)

	err := r.db.WithContext(ctx).
		Model(&model.Group{}).
		Select("groups.*").
		Joins("JOIN group_members ON group_members.group_id = groups.id").
		Where("group_members.user_id = ?", userID).
		Order("groups.created_at DESC").
		Order("groups.id DESC").
		Find(&items).
		Error

	if err != nil {
		return nil, fmt.Errorf("list groups by user: %w", err)
	}

	return items, nil
}
