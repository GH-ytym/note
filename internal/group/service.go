package group

import (
	"context"
	apperrors "note/internal/errors"
	"note/internal/model"
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
	// 初始成员就是群主；GroupID 由 GORM 在创建群后自动填入。
	item := model.Group{
		Name:    name,
		OwnerID: ownerID,
		Members: []model.GroupMember{
			{UserID: ownerID},
		},
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
