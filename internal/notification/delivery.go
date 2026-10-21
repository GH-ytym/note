package notification

import (
	"context"
)

// 只依赖投递所需的方法，方便测试。
type SnapshotLoader interface {
	LoadSnapshot(ctx context.Context, userID uint) (Inbox, error)
}

type ChangeCache interface {
	ApplyChange(
		ctx context.Context,
		userID uint,
		change Change,
	) (ApplyResult, error)

	InitSnapshot(
		ctx context.Context,
		userID uint,
		inbox Inbox,
	) (bool, error)
}

type Delivery struct {
	repo  SnapshotLoader
	cache ChangeCache
}

func NewDelivery(
	repo SnapshotLoader,
	cache ChangeCache,
) *Delivery {
	return &Delivery{
		repo:  repo,
		cache: cache,
	}
}
