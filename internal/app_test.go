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

	if err := db.AutoMigrate(
		&model.Todo{},
		&model.TodoDate{},
		&model.TodoCompletion{},
	); err != nil {
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
	item := model.Todo{
		Content:    "SQLite integration test",
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

	duplicate := model.Todo{
		Content:    item.Content,
		Color:      "#F3B51B",
		StartsAt:   &startsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyNone,
	}
	if err := db.Create(&duplicate).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate content error = %v, want %v", err, gorm.ErrDuplicatedKey)
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
