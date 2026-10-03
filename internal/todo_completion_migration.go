package internal

import (
	"fmt"

	"gorm.io/gorm"
)

// 独立迁移完成表，避免 AutoMigrate 沿关联重建有数据的 todos。
func migrateTodoCompletionSchema(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("todo_completions") {
			if err := tx.Exec(`CREATE TABLE todo_completions (
				id integer PRIMARY KEY AUTOINCREMENT,
				todo_id integer NOT NULL,
				occurs_on date NOT NULL,
				records text NOT NULL DEFAULT '[]',
				CONSTRAINT fk_todo_completions_todo
					FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
			)`).Error; err != nil {
				return err
			}
		} else {
			if !tx.Migrator().HasColumn("todo_completions", "records") {
				var count int64
				if err := tx.Table("todo_completions").Count(&count).Error; err != nil {
					return err
				}
				// 旧记录只描述整体完成，无法推断是谁完成的；保留原数据等待回填。
				if count > 0 {
					return fmt.Errorf("todo_completions 有 %d 条旧整体完成记录，需要先明确用户并回填 records", count)
				}
				if err := tx.Exec("ALTER TABLE todo_completions ADD COLUMN records text NOT NULL DEFAULT '[]'").Error; err != nil {
					return err
				}
			}
			if tx.Migrator().HasColumn("todo_completions", "completed_at") {
				// 仅在旧表为空时移除旧列；已回填的混合结构保留原始时间。
				var count int64
				if err := tx.Table("todo_completions").Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("todo_completions 仍有旧 completed_at 列，需要完成旧记录迁移后移除该列")
				}
				if err := tx.Exec("ALTER TABLE todo_completions DROP COLUMN completed_at").Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_todo_completion ON todo_completions(todo_id, occurs_on)").Error
	})
}
