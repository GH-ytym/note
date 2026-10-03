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
	Create(
		ctx context.Context,
		ownerID uint,
		name string,
	) (model.Group, error)
	MyGroups(
		ctx context.Context,
		userID uint,
	) ([]model.Group, error)
	GetInviteCode(
		ctx context.Context,
		groupID uint,
		userID uint,
	) (string, error)
	RefreshInviteCode(
		ctx context.Context,
		groupID uint,
		userID uint,
	) (string, error)
	JoinGroup(
		ctx context.Context,
		groupID uint,
		userID uint,
		code string,
	) error
	QuitGroup(
		ctx context.Context,
		groupID uint,
		userID uint,
	) error
}
type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
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
	//生成邀请码
	code, err := utils.GenerateInviteCode()
	if err != nil {
		return model.Group{}, err
	}

	// 初始成员就是群主；GroupID 由 GORM 在创建群后自动填入。
	item := model.Group{
		Name:    name,
		OwnerID: ownerID,
		Members: []model.GroupMember{
			{UserID: ownerID},
		},
		Code: code,
	}
	if err := s.repo.Create(ctx, &item); err != nil {
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

func (s *service) GetInviteCode(ctx context.Context, groupID uint, userID uint) (string, error) {
	//TODO implement me
	if groupID == 0 {
		return "", apperrors.ErrGroupNotFound
	}
	if userID == 0 {
		return "", apperrors.ErrGroupUnauthenticated
	}

	return s.repo.GetInviteCode(ctx, groupID, userID)
}

func (s *service) RefreshInviteCode(
	ctx context.Context,
	groupID uint,
	userID uint,
) (string, error) {
	if userID == 0 {
		return "", apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return "", apperrors.ErrGroupNotFound
	}

	var code string

	err := retry.Do(ctx, func() error {
		// 每次尝试都生成新的候选码。
		candidate, err := utils.GenerateInviteCode()
		if err != nil {
			return err
		}

		//重试就是这个replace
		if err := s.repo.ReplaceInviteCode(
			ctx,
			groupID,
			userID,
			candidate,
		); err != nil {
			return err
		}

		code = candidate
		return nil //所有return只是结束这一次重试
		//当然如果return nil，整个retry同样会return nil
	}, retry.Options{
		//配置项
		MaxAttempts:  5,
		InitialDelay: 0,
		ShouldRetry: func(err error) bool {
			// 只在候选码与本群原码相同时重新生成。
			return errors.Is(
				err,
				apperrors.ErrGroupInviteCodeConflict,
			)
		},
	})
	if err != nil {
		return "", err
	}

	return code, nil
}

func (s *service) JoinGroup(
	ctx context.Context,
	groupID uint,
	userID uint,
	code string,
) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return apperrors.ErrGroupNotFound
	}

	//邀请码的格式错误应该在前端就被拦下
	////就算打入后端的请求种邀请码真的错了，进入repo一样能被拦截
	return s.repo.Join(ctx, groupID, userID, code)
}

func (s *service) QuitGroup(
	ctx context.Context,
	groupID uint,
	userID uint,
) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if groupID == 0 {
		return apperrors.ErrGroupNotFound
	}

	return s.repo.Quit(ctx, groupID, userID)
}
