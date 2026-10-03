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
	GetInviteCode(ctx context.Context, groupID uint, userID uint) (string, error)
	ReplaceInviteCode(
		ctx context.Context,
		groupID uint,
		userID uint,
		code string,
	) error
	Join(ctx context.Context, groupID uint, userID uint, code string) error
}

type gormRepository struct {
	db *gorm.DB
}

func (r *gormRepository) Join(ctx context.Context, groupID uint, userID uint, code string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 找到指定群，只和这个群的当前邀请码比较
		var item model.Group
		//这里会拿出code
		err := tx.Select("id", "code").First(&item, groupID).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find group before joining: %w", err)
		}
		if item.Code == "" || item.Code != code {
			return apperrors.ErrGroupInviteInvalid
		}

		// 2. JWT 对应的账号必须仍然存在
		var user model.User
		err = tx.Select("id").First(&user, userID).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupUnauthenticated
		}
		if err != nil {
			return fmt.Errorf("find joining user: %w", err)
		}

		//如果已经在群里了，应该直接返回
		var count int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check existing membership: %w", err)
		}
		if count > 0 {
			return nil
		}

		//不在群里就加入
		// 查询本群已有 Todo
		var todos []model.Todo
		if err := tx.Select("id", "creator_id").
			Where("group_id = ?", groupID).
			Find(&todos).Error; err != nil {
			return fmt.Errorf("find group todos: %w", err)
		}

		// 给新加入的成员初始化权限。
		permissions := make([]model.TodoMember, 0, len(todos))

		for _, todo := range todos {
			role := model.TodoViewer
			if todo.CreatorID == userID {
				role = model.TodoEditor
			}

			permissions = append(permissions, model.TodoMember{
				TodoID: todo.ID,
				UserID: userID,
				Role:   role,
			})
		}

		//如果没有要创建的权限记录，就直接结束事务中的操作，不执行批量插入
		//(群里一个todo都没有)
		//否则反而会导致下面的操作失败引发回滚
		if len(permissions) == 0 {
			return nil
		}

		//插入member
		member := model.GroupMember{
			GroupID: groupID,
			UserID:  userID,
		}
		if err := tx.Omit("User").Create(&member).Error; err != nil {
			return fmt.Errorf("create group membership: %w", err)
		}

		return nil
	})
}

func (r *gormRepository) GetInviteCode(ctx context.Context, groupID uint, userID uint) (string, error) {
	var code string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group model.Group
		err := tx.First(&group, "id = ?", groupID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}

		//只要user在群里就可以拿到邀请码，但只有群主可以刷新
		var cnt int64
		if err := tx.Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id = ?",
				groupID,
				userID,
			).
			Count(&cnt).Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}
		if cnt == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		// 旧群还没有回填邀请码时，不返回一个无效的空码。
		if group.Code == "" {
			return fmt.Errorf("group %d has no invite code", groupID)
		}

		// 检查全部通过后才返回邀请码。
		code = group.Code
		return nil
	})
	if err != nil {
		return "", err
	}
	return code, err
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

func (r *gormRepository) ReplaceInviteCode(
	ctx context.Context,
	groupID uint,
	userID uint,
	code string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查询当前群主和邀请码。
		var item model.Group
		err := tx.Select("id", "owner_id", "code").
			First(&item, groupID).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find group before refreshing invite: %w", err)
		}

		// 2. 只有群主可以刷新。
		if item.OwnerID != userID {
			return apperrors.ErrGroupAccessDenied
		}

		// 3. 群主必须仍在群内。
		var count int64
		if err := tx.Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id = ?",
				groupID,
				userID,
			).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}
		if count == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		// 4. 随机生成也可能恰好与原码相同，要求重新生成。
		if code == item.Code {
			return apperrors.ErrGroupInviteCodeConflict
		}

		// 5. 只替换邀请码；GORM 会同时维护 UpdatedAt。
		result := tx.Model(&model.Group{}).
			Where("id = ?", groupID).
			Update("code", code)

		// 不同群允许同码，数据库保存错误按原原因返回。
		if result.Error != nil {
			return fmt.Errorf("replace group invite code: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return apperrors.ErrGroupNotFound
		}

		return nil
	})
}
