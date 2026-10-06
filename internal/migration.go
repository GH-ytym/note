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
	// User 没有关联旧日程表，可以单独同步，避免重建已有的 Todo/Event 表。
	if err := migrateAuthSchema(db); err != nil {
		return fmt.Errorf("migrate users schema: %w", err)
	}

	// 群组依赖已存在的用户表；成员关系独立保存，支持一个用户加入多个群。
	if err := migrateGroupSchema(db); err != nil {
		return fmt.Errorf("migrate group schema: %w", err)
	}
	if err := migrateGroupJoinSchema(db); err != nil {
		return fmt.Errorf("migrate group join request schema: %w", err)
	}
	if err := migrateNotificationSchema(db); err != nil {
		return fmt.Errorf("migrate notification schema: %w", err)
	}
	if err := migrateOutboxSchema(db); err != nil {
		return fmt.Errorf("migrate outbox schema: %w", err)
	}

	if err := migrateTodoSchema(db); err != nil {
		return err
	}
	if err := migrateTodoCompletionSchema(db); err != nil {
		return fmt.Errorf("migrate todo completions: %w", err)
	}
	if db.Migrator().HasColumn("todos", "all_done") {
		if err := db.Exec("ALTER TABLE todos DROP COLUMN all_done").Error; err != nil {
			return fmt.Errorf("remove todos.all_done: %w", err)
		}
	}

	if err := migrateEventSchema(db); err != nil {
		return err
	}

	return nil
}

func migrateEventSchema(db *gorm.DB) error {
	var count int64
	if db.Migrator().HasTable(&model.Event{}) {
		if err := db.Model(&model.Event{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			if !db.Migrator().HasColumn("events", "group_id") || !db.Migrator().HasColumn("events", "creator_id") {
				return fmt.Errorf("events 表有 %d 条旧数据，需要先明确并回填 group_id 和 creator_id", count)
			}
			var unassigned int64
			if err := db.Raw(`SELECT COUNT(*) FROM events e
				LEFT JOIN groups g ON g.id = e.group_id LEFT JOIN users u ON u.id = e.creator_id
				WHERE g.id IS NULL OR u.id IS NULL`).Scan(&unassigned).Error; err != nil {
				return err
			}
			if unassigned > 0 {
				return fmt.Errorf("events 表有 %d 条数据的群组或创建者无效，需要先回填归属", unassigned)
			}
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable(&model.Event{}) {
			if err := tx.Migrator().CreateTable(&model.Event{}); err != nil {
				return fmt.Errorf("create event schema: %w", err)
			}
		} else {
			// 空的旧表可以补归属列；有旧数据时上面已要求人工明确归属。
			for _, column := range []struct{ name, definition string }{
				{"group_id", "integer NOT NULL REFERENCES groups(id) ON DELETE RESTRICT"},
				{"creator_id", "integer NOT NULL REFERENCES users(id) ON DELETE RESTRICT"},
			} {
				if !tx.Migrator().HasColumn("events", column.name) {
					if err := tx.Exec("ALTER TABLE events ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
						return err
					}
				}
			}
		}

		// Avoid rebuilding a populated SQLite events table. The SQLite migrator can
		// omit old columns while copying data into its temporary table.
		if !tx.Migrator().HasColumn(&model.Event{}, "RepeatMode") {
			if err := tx.Exec("ALTER TABLE `events` ADD COLUMN `repeat_mode` text NOT NULL DEFAULT 'once'").Error; err != nil {
				return fmt.Errorf("add events.repeat_mode: %w", err)
			}
		}

		for _, child := range []any{&model.EventDate{}, &model.EventMember{}} {
			if !tx.Migrator().HasTable(child) {
				if err := tx.Migrator().CreateTable(child); err != nil {
					return fmt.Errorf("create event child schema: %w", err)
				}
			}
		}
		for _, statement := range []string{
			"CREATE INDEX IF NOT EXISTS idx_events_group_id ON events(group_id)",
			"CREATE INDEX IF NOT EXISTS idx_events_creator_id ON events(creator_id)",
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		// 只补缺失授权，重复启动不会覆盖创建者已设置的 editor/viewer。
		return tx.Exec(`INSERT INTO event_members (event_id, user_id, role, created_at, updated_at)
		SELECT e.id, gm.user_id, CASE WHEN gm.user_id = e.creator_id THEN 'editor' ELSE 'viewer' END,
		CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM events e JOIN group_members gm ON gm.group_id = e.group_id
		WHERE NOT EXISTS (SELECT 1 FROM event_members em WHERE em.event_id = e.id AND em.user_id = gm.user_id)`).Error
	})
}

func migrateTodoSchema(db *gorm.DB) error {
	// 旧单机 Todo 没有所属群组和创建者，不能自动编造归属。
	// 空表可以同步完整结构；有数据时必须先具备明确的归属。
	var count int64
	if db.Migrator().HasTable(&model.Todo{}) {
		if err := db.Model(&model.Todo{}).Count(&count).Error; err != nil {
			return fmt.Errorf("count todos before migration: %w", err)
		}
		if count > 0 {
			if !db.Migrator().HasColumn(&model.Todo{}, "GroupID") || !db.Migrator().HasColumn(&model.Todo{}, "CreatorID") {
				return fmt.Errorf("todos 表有 %d 条旧数据，需要先明确并回填 group_id 和 creator_id", count)
			}
			var unassigned int64
			if err := db.Raw(`SELECT COUNT(*) FROM todos t
				LEFT JOIN groups g ON g.id = t.group_id
				LEFT JOIN users u ON u.id = t.creator_id
				WHERE g.id IS NULL OR u.id IS NULL`).Scan(&unassigned).Error; err != nil {
				return fmt.Errorf("check todo ownership: %w", err)
			}
			if unassigned > 0 {
				return fmt.Errorf("todos 表有 %d 条数据的群组或创建者无效，需要先回填归属", unassigned)
			}
		}
	}

	if count == 0 {
		return db.Transaction(func(tx *gorm.DB) error {
			// CreateTable 只建指定表，不像 AutoMigrate 那样回头升级关联的 groups。
			if !tx.Migrator().HasTable(&model.Todo{}) {
				if err := tx.Migrator().CreateTable(&model.Todo{}); err != nil {
					return fmt.Errorf("create todo schema: %w", err)
				}
			} else {
				// 空的旧单机表可以直接补列，不需要编造历史数据归属。
				for _, column := range []struct{ name, definition string }{
					{"group_id", "integer NOT NULL REFERENCES groups(id) ON DELETE RESTRICT"},
					{"creator_id", "integer NOT NULL REFERENCES users(id) ON DELETE RESTRICT"},
					{"title", "text NOT NULL DEFAULT ''"},
				} {
					if !tx.Migrator().HasColumn("todos", column.name) {
						if err := tx.Exec("ALTER TABLE todos ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
							return fmt.Errorf("add todos.%s: %w", column.name, err)
						}
					}
				}
			}
			for _, child := range []any{&model.TodoDate{}, &model.TodoMember{}} {
				if !tx.Migrator().HasTable(child) {
					if err := tx.Migrator().CreateTable(child); err != nil {
						return fmt.Errorf("create todo child schema: %w", err)
					}
				}
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_todos_group_id ON todos(group_id)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_todos_creator_id ON todos(creator_id)").Error; err != nil {
				return err
			}
			return dropLegacyTodoUniqueIndexes(tx)
		})
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

	// Do not AutoMigrate models related to a legacy todos table here. This
	// SQLite migrator follows the relation back to Todo, rebuilds the parent
	// table and can omit pointer-backed fields while copying legacy rows.
	return db.Transaction(func(tx *gorm.DB) error {
		if err := dropLegacyTodoUniqueIndexes(tx); err != nil {
			return err
		}
		// 显式建权限表，避免递归 AutoMigrate 重建有数据的 todos。
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS todo_members (
			todo_id integer NOT NULL,
			user_id integer NOT NULL,
			role text NOT NULL CHECK (role IN ('viewer','editor')),
			created_at datetime,
			updated_at datetime,
			PRIMARY KEY (todo_id, user_id),
			FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`).Error; err != nil {
			return fmt.Errorf("create todo member schema: %w", err)
		}
		if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_todo_members_user_id ON todo_members(user_id)").Error; err != nil {
			return err
		}
		// 只补缺失记录，不覆盖已由创建者设置的 viewer/editor。
		if err := tx.Exec(`INSERT INTO todo_members (todo_id, user_id, role, created_at, updated_at)
			SELECT t.id, gm.user_id,
				CASE WHEN gm.user_id = t.creator_id THEN 'editor' ELSE 'viewer' END,
				CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM todos t JOIN group_members gm ON gm.group_id = t.group_id
			WHERE NOT EXISTS (SELECT 1 FROM todo_members tm
				WHERE tm.todo_id = t.id AND tm.user_id = gm.user_id)`).Error; err != nil {
			return fmt.Errorf("initialize missing todo permissions: %w", err)
		}
		return nil
	})
}

func dropLegacyTodoUniqueIndexes(db *gorm.DB) error {
	for _, name := range []string{"idx_todos_content", "idx_todos_title"} {
		if db.Migrator().HasIndex(&model.Todo{}, name) {
			if err := db.Migrator().DropIndex(&model.Todo{}, name); err != nil {
				return fmt.Errorf("drop legacy todo index %s: %w", name, err)
			}
		}
	}
	return nil
}
