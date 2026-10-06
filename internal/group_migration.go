package internal

import (
	"fmt"

	"note/internal/model"
	"note/internal/utils"

	"gorm.io/gorm"
)

func migrateGroupSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable(&model.Group{}) {
			return tx.AutoMigrate(&model.Group{}, &model.GroupMember{})
		}

		// 有数据的父表显式升级，避免重建 groups 时级联影响成员和 Todo。
		if !tx.Migrator().HasColumn(&model.Group{}, "Policy") {
			// 旧群全部保留原来的直接入群行为；CHECK 与新模型一致。
			if err := tx.Exec(`ALTER TABLE groups
				ADD COLUMN policy text NOT NULL DEFAULT 'public'
				CONSTRAINT ck_groups_policy
				CHECK (policy IN ('restricted','public','approval','personal'))`).Error; err != nil {
				return fmt.Errorf("add groups.policy: %w", err)
			}
		}
		if !tx.Migrator().HasColumn(&model.Group{}, "Code") {
			if err := tx.Exec("ALTER TABLE groups ADD COLUMN code text NOT NULL DEFAULT ''").Error; err != nil {
				return fmt.Errorf("add groups.code: %w", err)
			}
		}

		// 移除旧模型建立的唯一索引；仅删除模型标签不会删除数据库索引。
		if tx.Migrator().HasIndex(&model.Group{}, "idx_groups_code") {
			if err := tx.Migrator().DropIndex(&model.Group{}, "idx_groups_code"); err != nil {
				return fmt.Errorf("drop unique group invite index: %w", err)
			}
		}

		// 只给没有邀请码的旧群补码，不覆盖群主已经分享的有效邀请码。
		var groups []model.Group
		if err := tx.Select("id").Where("code = '' OR code IS NULL").Find(&groups).Error; err != nil {
			return fmt.Errorf("find groups without invite codes: %w", err)
		}
		for _, item := range groups {
			code, err := utils.GenerateInviteCode()
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Group{}).Where("id = ?", item.ID).UpdateColumn("code", code).Error; err != nil {
				return fmt.Errorf("backfill group %d invite code: %w", item.ID, err)
			}
		}

		// 显式建成员表，避免关联迁移回头重建已有的 groups。
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS group_members (
			group_id integer NOT NULL,
			user_id integer NOT NULL,
			joined_at datetime,
			PRIMARY KEY (group_id, user_id),
			CONSTRAINT fk_groups_members
				FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_members_user
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
		)`).Error; err != nil {
			return fmt.Errorf("create group member schema: %w", err)
		}
		return tx.Exec("CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id)").Error
	})
}

func migrateGroupJoinSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&model.GroupJoinRequest{}) {
			return nil
		}
		// 用户、群组表已存在，只创建申请表，不递归升级父表。
		// 表、外键和索引在同一事务中创建，失败后可在下次启动重试。
		return tx.Migrator().CreateTable(&model.GroupJoinRequest{})
	})
}

func migrateNotificationSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&model.Notification{}) {
			return nil
		}
		// 用户、群组、申请表已存在，只创建通知表，避免重建父表。
		return tx.Migrator().CreateTable(&model.Notification{})
	})
}

func migrateOutboxSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&model.Outbox{}) {
			return nil
		}
		// 只建投递任务表；重复启动保留已有的待投递和已投递任务。
		return tx.Migrator().CreateTable(&model.Outbox{})
	})
}
