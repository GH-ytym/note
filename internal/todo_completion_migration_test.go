package internal

import (
	"context"
	"testing"
	"time"

	"note/internal/model"
)

func TestTodoCompletionMigrationLegacyData(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			db, service, item, owner, _, date := completionFixture(t)
			for _, sql := range []string{
				"DROP TABLE todo_completions",
				`CREATE TABLE todo_completions (
					id integer PRIMARY KEY AUTOINCREMENT, todo_id integer NOT NULL,
					occurs_on date NOT NULL, completed_at datetime NOT NULL,
					FOREIGN KEY(todo_id) REFERENCES todos(id) ON DELETE CASCADE)`,
				"ALTER TABLE todos ADD COLUMN all_done numeric NOT NULL DEFAULT 0",
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			if populated {
				if err := db.Exec("INSERT INTO todo_completions(todo_id, occurs_on, completed_at) VALUES (?, ?, ?)", item.ID, date, date.Add(time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
				if err := migrateDatabase(db); err == nil {
					t.Fatal("cannot infer user for old global completion")
				}
				if db.Migrator().HasColumn("todo_completions", "records") || !db.Migrator().HasColumn("todo_completions", "completed_at") {
					t.Fatal("failed migration altered legacy completion schema")
				}
				var count int64
				if err := db.Table("todo_completions").Where("todo_id = ? AND completed_at = ?", item.ID, date.Add(time.Hour)).Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("legacy completion was lost: %d %v", count, err)
				}
				return
			}
			if err := migrateDatabase(db); err != nil {
				t.Fatal(err)
			}
			if !db.Migrator().HasColumn("todo_completions", "records") || db.Migrator().HasColumn("todo_completions", "completed_at") || db.Migrator().HasColumn("todos", "all_done") {
				t.Fatal("legacy columns not migrated")
			}
			if err := service.SetOccurrenceDone(context.Background(), item.ID, owner.ID, date, true); err != nil {
				t.Fatal(err)
			}
			if err := migrateDatabase(db); err != nil {
				t.Fatal(err)
			}
			var completion model.TodoCompletion
			if err := db.Where("todo_id = ?", item.ID).First(&completion).Error; err != nil || len(completion.Records) != 1 || completion.Records[0].UserID != owner.ID {
				t.Fatalf("repeated migration lost JSON records: %#v %v", completion, err)
			}
		})
	}
}
