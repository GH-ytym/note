package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"note/internal/model"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestSQLiteDatabase(t *testing.T) {
	databasePath, err := prepareDatabasePath(
		filepath.Join(t.TempDir(), "nested", "note.db"),
	)
	if err != nil {
		t.Fatalf("prepare database path: %v", err)
	}

	dsn := sqliteDSN(databasePath)
	if !strings.HasPrefix(dsn, "file:"+filepath.ToSlash(databasePath)+"?") {
		t.Fatalf("unexpected SQLite DSN: %s", dsn)
	}

	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := migrateDatabase(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	var foreignKeys int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign key enforcement is disabled: %d", foreignKeys)
	}

	startsAt := time.Date(2026, time.August, 30, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	content := "SQLite integration test content"
	item := model.Todo{
		Title:      "SQLite integration test",
		Content:    &content,
		Color:      "#5B8DEF",
		StartsAt:   &startsAt,
		RepeatMode: model.RepeatCustom,
		NotifyMode: model.NotifyPopup,
		CustomDates: []model.TodoDate{
			{Date: time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC)},
		},
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create todo: %v", err)
	}

	var loaded model.Todo
	if err := db.Preload("CustomDates").First(&loaded, item.ID).Error; err != nil {
		t.Fatalf("load todo: %v", err)
	}
	if loaded.StartsAt == nil || !loaded.StartsAt.Equal(startsAt) {
		t.Fatalf("starts_at did not round-trip: got %v, want %v", loaded.StartsAt, startsAt)
	}
	if len(loaded.CustomDates) != 1 {
		t.Fatalf("custom dates did not round-trip: got %d", len(loaded.CustomDates))
	}

	sameContent := model.Todo{
		Title:      "Same content, different title",
		Content:    item.Content,
		Color:      "#F3B51B",
		StartsAt:   &startsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyNone,
	}
	if err := db.Create(&sameContent).Error; err != nil {
		t.Fatalf("same content with different title should be allowed: %v", err)
	}

	differentContent := "Different content"
	duplicate := model.Todo{
		Title:      item.Title,
		Content:    &differentContent,
		Color:      "#F3B51B",
		StartsAt:   &startsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyNone,
	}
	if err := db.Create(&duplicate).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate title error = %v, want %v", err, gorm.ErrDuplicatedKey)
	}

	completion := model.TodoCompletion{
		TodoID:      item.ID,
		OccursOn:    time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC),
		CompletedAt: time.Now(),
	}
	if err := db.Create(&completion).Error; err != nil {
		t.Fatalf("create completion: %v", err)
	}
	if err := db.Delete(&item).Error; err != nil {
		t.Fatalf("delete todo: %v", err)
	}

	var dateCount int64
	if err := db.Model(&model.TodoDate{}).Where("todo_id = ?", item.ID).Count(&dateCount).Error; err != nil {
		t.Fatalf("count custom dates: %v", err)
	}
	var completionCount int64
	if err := db.Model(&model.TodoCompletion{}).Where("todo_id = ?", item.ID).Count(&completionCount).Error; err != nil {
		t.Fatalf("count completions: %v", err)
	}
	if dateCount != 0 || completionCount != 0 {
		t.Fatalf("cascade delete failed: dates=%d completions=%d", dateCount, completionCount)
	}

	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}

func TestMigrateLegacyTodoSchema(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy-note.db")
	db, err := gorm.Open(
		gormlite.Open(sqliteDSN(databasePath)),
		&gorm.Config{TranslateError: true},
	)
	if err != nil {
		t.Fatalf("open legacy SQLite database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get legacy sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Exec(`
		CREATE TABLE todos (
			id integer PRIMARY KEY AUTOINCREMENT,
			content text NOT NULL,
			color text NOT NULL DEFAULT '#F3B51B',
			starts_at datetime NOT NULL,
			repeat_mode text NOT NULL DEFAULT 'once',
			notify_mode text NOT NULL DEFAULT 'none',
			all_done numeric NOT NULL DEFAULT false,
			created_at datetime,
			updated_at datetime,
			version integer NOT NULL DEFAULT 1
		)
	`).Error; err != nil {
		t.Fatalf("create legacy todos table: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_todos_content ON todos(content)").Error; err != nil {
		t.Fatalf("create legacy content index: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO todos (content, color, starts_at, repeat_mode, notify_mode, all_done, version)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "legacy content", "#F3B51B", time.Now(), model.RepeatOnce, model.NotifyNone, false, 1).Error; err != nil {
		t.Fatalf("insert legacy todo: %v", err)
	}

	if err := migrateDatabase(db); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatalf("repeat legacy migration: %v", err)
	}

	var loaded model.Todo
	if err := db.First(&loaded).Error; err != nil {
		t.Fatalf("load migrated todo: %v", err)
	}
	if loaded.Title != "legacy content" {
		t.Fatalf("migrated title = %q, want legacy content", loaded.Title)
	}
	if loaded.Content == nil || *loaded.Content != "legacy content" {
		t.Fatalf("migrated content = %v, want legacy content", loaded.Content)
	}

	type indexInfo struct {
		Name   string `gorm:"column:name"`
		Unique int    `gorm:"column:unique"`
	}
	var indexes []indexInfo
	if err := db.Raw("PRAGMA index_list('todos')").Scan(&indexes).Error; err != nil {
		t.Fatalf("list migrated indexes: %v", err)
	}
	foundTitleIndex := false
	for _, index := range indexes {
		if index.Name == "idx_todos_content" {
			t.Fatal("legacy content index still exists")
		}
		if index.Name == "idx_todos_title" && index.Unique == 1 {
			foundTitleIndex = true
		}
	}
	if !foundTitleIndex {
		t.Fatal("unique title index was not created")
	}

	sameContent := model.Todo{
		Title:      "different title",
		Content:    loaded.Content,
		Color:      "#F3B51B",
		StartsAt:   loaded.StartsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyNone,
	}
	if err := db.Create(&sameContent).Error; err != nil {
		t.Fatalf("same content should be allowed after migration: %v", err)
	}

	differentContent := "different content"
	duplicateTitle := model.Todo{
		Title:      loaded.Title,
		Content:    &differentContent,
		Color:      "#F3B51B",
		StartsAt:   loaded.StartsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyNone,
	}
	if err := db.Create(&duplicateTitle).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate title error = %v, want %v", err, gorm.ErrDuplicatedKey)
	}
}
