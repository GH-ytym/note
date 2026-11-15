package notification

import (
	"context"
	apperrors "note/internal/errors"
)

// Service 是通知列表的业务入口，分页读取和恢复由 Reader 完成。
type Service interface {
	MarkRead(ctx context.Context, uid uint, ids []uint) error
	List(
		ctx context.Context,
		userID uint,
		before uint,
		limit uint,
	) (Page, error)

	Snapshot(
		ctx context.Context,
		uid uint,
	) (Inbox, error)

	SnapshotPage(
		ctx context.Context,
		uid uint,
		before uint,
		limit uint,
	) (SnapshotPage, error)

	Revision(ctx context.Context, uid uint) (uint64, error)
}

// MarkRead 将当前登录用户选中的通知批量标为已读。
func (s *service) MarkRead(ctx context.Context, uid uint, ids []uint) error {
	if uid == 0 {
		return apperrors.ErrNotificationUnauthenticated
	}
	return s.repo.MarkRead(ctx, uid, ids)
}

type service struct {
	reader *Reader
	repo   Repository
}

func NewService(reader *Reader, repo Repository) Service {
	return &service{
		reader: reader,
		repo:   repo,
	}
}

// 用redis的
func (s *service) List(
	ctx context.Context,
	userID uint,
	before uint,
	limit uint,
) (Page, error) {
	if userID == 0 {
		return Page{}, apperrors.ErrNotificationUnauthenticated
	}
	return s.reader.List(ctx, userID, before, limit)
}

// 用数据库的
func (s *service) Snapshot(
	ctx context.Context,
	uid uint,
) (Inbox, error) {
	if uid == 0 {
		return Inbox{}, apperrors.ErrNotificationUnauthenticated
	}
	return s.repo.LoadSnapshot(ctx, uid)
}

// SnapshotPage 从数据库读取一页通知，以及当前用户的版本和全部未读数。
func (s *service) SnapshotPage(
	ctx context.Context,
	uid uint,
	before uint,
	limit uint,
) (SnapshotPage, error) {
	if uid == 0 {
		return SnapshotPage{}, apperrors.ErrNotificationUnauthenticated
	}

	return s.repo.LoadPage(ctx, uid, before, limit)
}

func (s *service) Revision(
	ctx context.Context,
	uid uint,
) (uint64, error) {
	if uid == 0 {
		return 0, apperrors.ErrNotificationUnauthenticated
	}

	return s.repo.Revision(ctx, uid)
}
