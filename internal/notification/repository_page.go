package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"note/internal/model"

	"gorm.io/gorm"
)

// 游标分页
type SnapshotPage struct {
	//但是revision和unreadcnt依旧是整个用户的，不会因为分页就算错
	//next看有没有下一页，没有就是0
	Revision    uint64 `json:"revision,string"`
	UnreadCount int64  `json:"unread_count"`
	Items       []Card `json:"items"`
	Next        uint   `json:"next"`
}

func (r *gormRepository) LoadPage(
	ctx context.Context,
	uid uint,
	before uint,
	limit uint,
) (SnapshotPage, error) {
	if uid == 0 {
		return SnapshotPage{}, errors.New("invalid uid")
	}
	if uint64(before) > uint64(1<<53-1) {
		return SnapshotPage{}, errors.New("invalid cursor")
	}
	if limit < 1 || limit > 100 {
		return SnapshotPage{}, errors.New("invalid limit")
	}

	page := SnapshotPage{
		Items: make([]Card, 0, int(limit)),
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//先查全部的revision unreadcnt
		var state model.NotificationState
		if err := tx.Where("user_id = ?", uid).First(&state).Error; err != nil {
			return fmt.Errorf("get notification state: %w", err)
		}
		page.Revision = state.Revision
		page.UnreadCount = state.UnreadCount

		//根据上一页最后一个通知的位置，开始这一页
		//先固定用户
		query := tx.Where("receiver_id=?", uid)
		if before != 0 {
			//再固定游标
			//这里id是 desc的,所以是从大到小不是从小到大拿
			//before就是上一页最后一条，也就是上一页最小的id
			query = query.Where("id<?", before)
		}
		//多拿一条看看有没有下一页
		var notice []model.Notification
		if err := preloadCardRelations(query).
			Order("id DESC").
			Limit(int(limit) + 1).
			Find(&notice).Error; err != nil {
			return fmt.Errorf("get notification list: %w", err)
		}

		//查出来发现notice超出limit（说明确实有下一页）
		if len(notice) > int(limit) {
			//把最后一条切了
			notice = notice[:int(limit)]
			//切完后的最后一条会成为下一页的起点，因为上面写的是开区间
			page.Next = notice[len(notice)-1].ID
		}
		for _, notice := range notice {
			card, err := NewCard(notice)
			if err != nil {
				return err
			}
			page.Items = append(page.Items, card)
		}

		return nil
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SnapshotPage{}, err
	}
	return page, nil
}
