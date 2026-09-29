package internal

import (
	"fmt"
	"gorm.io/gorm"
	"note/internal/model"
)

// 不给旧账号编造邮箱，也不删除旧用户；有旧数据时等待明确的回填方案。
func migrateAuthSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		migrator := tx.Migrator()
		if migrator.HasTable(&model.User{}) && (!migrator.HasColumn(&model.User{}, "Email") || !migrator.HasColumn(&model.User{}, "Suffix")) {
			var count int64
			if err := tx.Model(&model.User{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("users 表有 %d 个旧账号，需要先回填邮箱和 suffix", count)
			}
		}
		if err := tx.AutoMigrate(&model.User{}); err != nil {
			return err
		}
		if migrator.HasIndex(&model.User{}, "idx_users_username") {
			return migrator.DropIndex(&model.User{}, "idx_users_username")
		}
		return nil
	})
}
