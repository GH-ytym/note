package internal

import (
	"fmt"

	"note/internal/model"

	"gorm.io/gorm"
)

// migrateDatabase upgrades existing SQLite databases before synchronizing the
// final GORM schema. Each step is idempotent so an interrupted migration can be
// continued on the next application start.
func migrateDatabase(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Todo{}) {
		if err := db.AutoMigrate(
			&model.Todo{},
			&model.TodoDate{},
			&model.TodoCompletion{},
		); err != nil {
			return fmt.Errorf("auto migrate fresh schema: %w", err)
		}
		return nil
	}

	if !db.Migrator().HasColumn(&model.Todo{}, "Title") {
		// SQLite can add a NOT NULL column to a populated table when it has a
		// temporary non-NULL default. The service still rejects empty titles.
		if err := db.Exec("ALTER TABLE `todos` ADD COLUMN `title` text NOT NULL DEFAULT ''").Error; err != nil {
			return fmt.Errorf("add todos.title: %w", err)
		}
	}

	if err := db.Exec(`
			UPDATE todos
			SET title = content
			WHERE title IS NULL OR trim(title) = ''
		`).Error; err != nil {
		return fmt.Errorf("backfill todos.title: %w", err)
	}

	if db.Migrator().HasIndex(&model.Todo{}, "idx_todos_content") {
		if err := db.Migrator().DropIndex(&model.Todo{}, "idx_todos_content"); err != nil {
			return fmt.Errorf("drop legacy content index: %w", err)
		}
	}

	if !db.Migrator().HasIndex(&model.Todo{}, "idx_todos_title") {
		if err := db.Exec("CREATE UNIQUE INDEX `idx_todos_title` ON `todos` (`title`)").Error; err != nil {
			return fmt.Errorf("create unique title index: %w", err)
		}
	}

	// Do not AutoMigrate models related to a legacy todos table here. This
	// SQLite migrator follows the relation back to Todo, rebuilds the parent
	// table and can omit pointer-backed fields while copying legacy rows.
	return nil
}
