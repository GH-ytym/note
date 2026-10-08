package notification

import (
	"context"
	"fmt"
	apperrors "note/internal/errors"
)

type Service interface {
	List(ctx context.Context, userID uint) (Inbox, error)

	HistoryAfter(
		ctx context.Context,
		userID uint,
		afterID string,
	) ([]Event, error)
}

type service struct {
	repo    Repository
	history *RedisHistoryRepository
}

func NewService(
	repo Repository,
	history *RedisHistoryRepository,
) Service {
	return &service{
		repo:    repo,
		history: history,
	}
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

func (s *service) HistoryAfter(
	ctx context.Context,
	userID uint,
	afterID string,
) ([]Event, error) {
	if userID == 0 {
		return nil, apperrors.ErrNotificationUnauthenticated
	}
	if s.history == nil {
		return nil, fmt.Errorf("notification history is not configured")
	}

	return s.history.ReadAfter(ctx, userID, afterID)
}
