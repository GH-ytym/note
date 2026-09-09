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
	if err := migrateTodoSchema(db); err != nil {
		return err
	}

	if err := migrateEventSchema(db); err != nil {
		return err
	}

	return nil
}

func migrateEventSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Event{}) {
		if err := db.AutoMigrate(&model.Event{}, &model.EventDate{}); err != nil {
			return fmt.Errorf("auto migrate fresh event schema: %w", err)
		}
		return nil
	}

	// Avoid rebuilding a populated SQLite events table. The SQLite migrator can
	// omit old columns while copying data into its temporary table.
	if !db.Migrator().HasColumn(&model.Event{}, "RepeatMode") {
		if err := db.Exec("ALTER TABLE `events` ADD COLUMN `repeat_mode` text NOT NULL DEFAULT 'once'").Error; err != nil {
			return fmt.Errorf("add events.repeat_mode: %w", err)
		}
	}

	if !db.Migrator().HasTable(&model.EventDate{}) {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`
				CREATE TABLE event_dates (
					id integer PRIMARY KEY AUTOINCREMENT,
					event_id integer NOT NULL,
					date date NOT NULL,
					created_at datetime,
					CONSTRAINT fk_events_custom_dates
						FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
				)
			`).Error; err != nil {
				return err
			}
			return tx.Exec("CREATE UNIQUE INDEX idx_event_date ON event_dates(event_id, date)").Error
		}); err != nil {
			return fmt.Errorf("create event_dates: %w", err)
		}
	}

	return nil
}

func migrateTodoSchema(db *gorm.DB) error {
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
