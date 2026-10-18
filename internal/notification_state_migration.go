package internal

import (
	"fmt"

	"note/internal/model"

	"gorm.io/gorm"
)

// migrateNotificationStateSchema 只初始化缺失状态，不重置已分配的版本。
// 后续业务写入必须在同一事务中维护状态；启动迁移不是计数修复任务。
func migrateNotificationStateSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable(&model.NotificationState{}) {
			// 仅创建状态表，避免自动迁移关联的用户表。
			if err := tx.Migrator().CreateTable(&model.NotificationState{}); err != nil {
				return fmt.Errorf("create notification states: %w", err)
			}
		} else if !tx.Migrator().HasColumn(&model.NotificationState{}, "UnreadCount") {
			if err := tx.Exec(`ALTER TABLE notification_states
				ADD COLUMN unread_count integer NOT NULL DEFAULT 0`).Error; err != nil {
				return fmt.Errorf("add notification unread count: %w", err)
			}
			// 仅在首次补列时回填，保留已有 revision。
			if err := tx.Exec(`UPDATE notification_states SET unread_count = (
				SELECT COUNT(*) FROM notifications n
				WHERE n.receiver_id = notification_states.user_id AND n.read_at IS NULL
			)`).Error; err != nil {
				return fmt.Errorf("backfill notification unread counts: %w", err)
			}
		}

		// 为所有缺少状态的老用户补齐记录；有旧通知的保留真实未读数。
		// 新用户在注册事务中初始化为 0，不在首次通知或登录时补建。
		return tx.Exec(`INSERT INTO notification_states (user_id, revision, unread_count)
			SELECT u.id, 0,
				(SELECT COUNT(*) FROM notifications n
				 WHERE n.receiver_id = u.id AND n.read_at IS NULL)
			FROM users u
			WHERE NOT EXISTS (
				SELECT 1 FROM notification_states s WHERE s.user_id = u.id
			)`).Error
	})
}
