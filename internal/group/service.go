package group

import (
	"context"
	"errors"
	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/retry"
	"note/internal/utils"
	"strings"
	"unicode/utf8"
)

type Service interface {
	Lookup(ctx context.Context, userID uint, code string) (model.Group, error)
	Create(
		ctx context.Context,
		ownerID uint,
		name string,
	) (model.Group, error)
	MyGroups(
		ctx context.Context,
		userID uint,
	) ([]model.Group, error)
	JoinGroup(
		ctx context.Context,
		userID uint,
		code string,
	) (*model.GroupJoinRequest, error)
	QuitGroup(
		ctx context.Context,
		groupID uint,
		userID uint,
		target *uint,
	) error
	ListMembers(ctx context.Context, groupID, userID uint) ([]model.GroupMember, error)
	DismissGroup(ctx context.Context, groupID, userID uint) error
}
type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListMembers(ctx context.Context, groupID, userID uint) ([]model.GroupMember, error) {
	if userID == 0 {
		return nil, apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return nil, apperrors.ErrGroupNotFound
	}
	return s.repo.ListMembers(ctx, groupID, userID)
}

func (s *service) DismissGroup(ctx context.Context, groupID, userID uint) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return apperrors.ErrGroupNotFound
	}
	return s.repo.Dismiss(ctx, groupID, userID)
}

func (s *service) Create(
	ctx context.Context,
	ownerID uint,
	name string,
) (model.Group, error) {
	if ownerID == 0 {
		return model.Group{}, apperrors.ErrGroupUnauthenticated
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		return model.Group{}, apperrors.ErrGroupNameInvalid
	}
	var item model.Group
	err := retry.Do(ctx, func() error {
		code, err := utils.GenerateGroupCode()
		if err != nil {
			return err
		}
		// 每次重试重新构造，避免回滚后残留的主键和成员关联。
		item = model.Group{Name: name, OwnerID: ownerID, Policy: model.Public,
			Code: code, Members: []model.GroupMember{{UserID: ownerID}}}
		return s.repo.Create(ctx, &item)
	}, retry.Options{MaxAttempts: 5, ShouldRetry: func(err error) bool {
		return errors.Is(err, apperrors.ErrGroupCodeConflict)
	}})
	if err != nil {
		return model.Group{}, err
	}

	return item, nil
}

func (s *service) MyGroups(
	ctx context.Context,
	userID uint,
) ([]model.Group, error) {
	if userID == 0 {
		return nil, apperrors.ErrGroupUnauthenticated
	}
	return s.repo.ListByUserID(ctx, userID)
}

func (s *service) JoinGroup(
	ctx context.Context,
	userID uint,
	code string,
) (*model.GroupJoinRequest, error) {
	if userID == 0 {
		return nil, apperrors.ErrGroupUnauthenticated
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	if !validGroupCode(code) {
		return nil, apperrors.ErrGroupCodeFormat
	}
	pending, _, err := s.repo.Join(ctx, userID, code)
	if err != nil {
		return nil, err
	}

	// repo 已将成员／申请和通知提交到数据库，返回入群结果给申请人。
	return pending, nil
}

func (s *service) QuitGroup(
	ctx context.Context,
	groupID uint,
	userID uint,
	target *uint,
) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return apperrors.ErrGroupNotFound
	}
	if target != nil && *target == 0 {
		return apperrors.ErrGroupTransferTargetInvalid
	}

	return s.repo.Quit(ctx, groupID, userID, target)
}

func (s *service) Lookup(ctx context.Context, userID uint, code string) (model.Group, error) {
	if userID == 0 {
		return model.Group{}, apperrors.ErrGroupUnauthenticated
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if !validGroupCode(code) {
		return model.Group{}, apperrors.ErrGroupCodeFormat
	}
	return s.repo.ByCode(ctx, code)
}
func validGroupCode(code string) bool {
	return len(code) == 6 && strings.IndexFunc(code, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z') }) < 0
}
