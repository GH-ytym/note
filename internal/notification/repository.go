package notification

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"note/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	ListByReceiverID(
		ctx context.Context,
		userID uint,
	) ([]model.Notification, int64, error)

	ByIDForReceiver(
		ctx context.Context,
		userID uint,
		notificationID uint,
	) (model.Notification, error)

	// 查询一批尚未投递的任务
	ListPendingOutbox(ctx context.Context) ([]model.Outbox, error)

	// Redis 投递成功后，记录成功时间
	MarkOutboxPublished(
		ctx context.Context,
		taskID uint,
		publishedAt time.Time,
	) error
}

type gormRepository struct {
	db *gorm.DB
}

func (r *gormRepository) MarkOutboxPublished(
	ctx context.Context,
	taskID uint,
	publishedAt time.Time,
) error {
	err := r.db.WithContext(ctx).
		Model(&model.Outbox{}).
		Where(
			"id = ? AND published_at IS NULL",
			taskID,
		).
		Update("published_at", publishedAt.UTC()). //现在有published_at了
		Error

	if err != nil {
		return fmt.Errorf("mark outbox task published: %w", err)
	}

	return nil
}

func (r *gormRepository) ListPendingOutbox(ctx context.Context) ([]model.Outbox, error) {
	ob := make([]model.Outbox, 0)
	err := r.db.WithContext(ctx).
		Where("published_at IS NULL"). //还没投递出去的
		Order("id ASC").               //后台投递应当以旧任务优先，也就是id小的优先
		Limit(100).Find(&ob).Error
	if err != nil {
		return nil, err
	}
	return ob, nil
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// 两种查询都加载卡片需要的关联资料。
// 这个函数只组装查询，Find 或 First 才执行查询。
// 负责preload actor group joinRequest
// Actor 和 Group 只填充 Select 指定的字段，其余字段保持 Go 零值。
// Receiver、JoinRequest.Sender／Receiver／Group 等关联不在这里加载。
func preloadCardRelations(query *gorm.DB) *gorm.DB {
	return query.
		Preload("Actor", func(db *gorm.DB) *gorm.DB {
			return db.Select(
				"id", "username", "suffix", "nickname", "avatar",
			)
		}).
		Preload("Group", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("JoinRequest")
}

// 某个用户的通知列表
func (r *gormRepository) ListByReceiverID(
	ctx context.Context,
	userID uint,
) ([]model.Notification, int64, error) {
	items := make([]model.Notification, 0)
	var unreadCount int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 统计这个用户的全部未读通知。
		if err := tx.Model(&model.Notification{}).
			Where("receiver_id = ? AND read_at IS NULL", userID).
			Count(&unreadCount).Error; err != nil {
			return err
		}

		query := tx.Where("receiver_id = ?", userID)

		// 与未读数量处于同一个数据库快照。
		return preloadCardRelations(query).
			Order("id DESC").
			Limit(50).
			Find(&items).Error
	}, &sql.TxOptions{ReadOnly: true})

	if err != nil {
		return nil, 0, err
	}

	return items, unreadCount, nil
}

// 属于某个用户的某条通知
func (r *gormRepository) ByIDForReceiver(
	ctx context.Context,
	userID uint,
	notificationID uint,
) (model.Notification, error) {
	var item model.Notification

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"id = ? AND receiver_id = ?",
			notificationID,
			userID,
		)

		return preloadCardRelations(query).
			First(&item).Error
	}, &sql.TxOptions{ReadOnly: true})

	if err != nil {
		return model.Notification{}, err
	}

	return item, nil
}
