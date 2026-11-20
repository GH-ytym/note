package group

import (
	"context"
	"errors"
	"fmt"
	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/notification"
	"time"

	"gorm.io/gorm"
)

// 审核入群申请
func (r *gormRepository) Review(
	ctx context.Context,
	uid, id uint,
	status model.Status,
) error {
	//要么同意要么拒绝
	if status != model.Accepted && status != model.Rejected {
		return apperrors.ErrGroupReviewInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//1.构建双方卡片的资料准备
		var req model.GroupJoinRequest
		err := tx.Preload("Group.Owner").Preload("Sender").
			First(&req, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupJoinRequestNotFound
		}
		if err != nil {
			return fmt.Errorf("load application: %w", err)
		}
		if req.Kind != model.Application {
			return apperrors.ErrGroupReviewInvalid
		}

		//看group对不对
		g := req.Group
		if g.OwnerID != uid {
			return apperrors.ErrGroupReviewDenied
		}
		if req.Status != model.Pending {
			return apperrors.ErrGroupReviewConflict
		}

		now := time.Now().UTC()
		//七天过期
		if !now.Before(req.CreatedAt.Add(7 * 24 * time.Hour)) {
			return apperrors.ErrGroupRequestExpired
		}

		//2.完成本次审核
		// 只允许 pending 转换一次；重复审核不会再次产生通知。
		res := tx.Model(&model.GroupJoinRequest{}).
			Where("id=? AND status=?", id, model.Pending).
			Updates(map[string]any{
				"status":     status,
				"handled_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperrors.ErrGroupReviewConflict
		}

		//updates之后还要改一下req
		req.Status = status
		req.HandledAt = &now

		// 3. 同意时添加成员
		if status == model.Accepted {
			var count int64
			if err := tx.Model(&model.GroupMember{}).
				Where("group_id = ? AND user_id = ?", g.ID, req.SenderID).
				Count(&count).Error; err != nil {
				return fmt.Errorf("check applicant membership: %w", err)
			}

			// 等待审核期间，用户可能已经通过其他方式入群
			if count == 0 {
				if err := r.addMember(tx, g.ID, req.SenderID); err != nil {
					return err
				}
			}
		}

		// 4. 找到原来发给群主的申请通知
		var msg model.Notification
		if err := tx.Where(
			"join_request_id = ? AND type = ?",
			id, model.JoinRequested,
		).First(&msg).Error; err != nil {
			return fmt.Errorf("load application notification: %w", err)
		}

		var delta int64

		// 同意、拒绝都会标记已读
		if msg.ReadAt == nil {
			if err := tx.Model(&model.Notification{}).
				Where("id = ?", msg.ID).
				Update("read_at", now).Error; err != nil {
				return fmt.Errorf("mark application read: %w", err)
			}

			msg.ReadAt = &now
			delta = -1
		}

		//前面preload过了
		msg.Actor = req.Sender
		msg.Group = g
		msg.JoinRequest = &req

		//构造卡片
		card, err := notification.NewCard(msg)
		if err != nil {
			return err
		}

		// 5.对于接收者，修改那张对应卡片，不发新的
		if _, err := notification.EnqueueChange(
			tx,
			msg.ReceiverID,
			[]notification.Card{card},
			nil,
			delta,
		); err != nil {
			return err
		}
		// 6. 给申请人保存一条未读的审核结果通知
		result := model.Notification{
			ReceiverID:    req.SenderID,
			ActorID:       uid,
			GroupID:       g.ID,
			JoinRequestID: &req.ID,
			Type:          model.JoinReviewed,
			CreatedAt:     now,
		}
		if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&result).Error; err != nil {
			return fmt.Errorf("create review result: %w", err)
		}
		result.Actor = g.Owner
		result.Group = g
		result.JoinRequest = &req
		card, err = notification.NewCard(result)
		if err != nil {
			return err
		}
		_, err = notification.EnqueueChange(tx, result.ReceiverID, []notification.Card{card}, nil, 1)
		return err
	})
}
