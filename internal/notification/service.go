package notification

import (
	"context"
	apperrors "note/internal/errors"
)

type Service interface {
	List(ctx context.Context, userID uint) (Inbox, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, userID uint) (Inbox, error) {
	if userID == 0 {
		return Inbox{}, apperrors.ErrNotificationUnauthenticated
	}
	notices, unreadCnt, err := s.repo.ListByReceiverID(ctx, userID)
	if err != nil {
		return Inbox{}, err
	}

	//转化为前端卡片
	cards := make([]Card, 0, len(notices))
	for _, n := range notices {
		card, err := NewCard(n)
		if err != nil {
			return Inbox{}, err
		}
		cards = append(cards, card)

	}
	return Inbox{
		Items:       cards,
		UnreadCount: unreadCnt,
	}, nil
}
