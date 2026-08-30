package todo

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"note/internal/model"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func testService(t *testing.T) TodoService {
	t.Helper()

	query := url.Values{}
	query.Add("_pragma", "foreign_keys(on)")
	query.Set("_txlock", "immediate")
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "todo.db")) + "?" + query.Encode()
	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&model.Todo{}, &model.TodoDate{}, &model.TodoCompletion{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return NewService(NewGORMRepository(db))
}

func createTestTodo(t *testing.T, service TodoService, title string, content *string) model.Todo {
	t.Helper()
	startsAt := time.Date(2026, time.August, 30, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	item, err := service.Create(context.Background(), CreateCommand{
		Title:      title,
		Content:    content,
		StartsAt:   &startsAt,
		RepeatMode: model.RepeatOnce,
		NotifyMode: model.NotifyPopup,
	})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}
	return item
}

func stringPointer(value string) *string {
	return &value
}

func TestCreateUsesTitleWhenContentIsMissingOrBlank(t *testing.T) {
	service := testService(t)

	missing := createTestTodo(t, service, "  没传内容  ", nil)
	if missing.Title != "没传内容" || missing.Content == nil || *missing.Content != "没传内容" {
		t.Fatalf("missing content todo = %#v", missing)
	}

	blank := createTestTodo(t, service, "空内容", stringPointer("   "))
	if blank.Content == nil || *blank.Content != "空内容" {
		t.Fatalf("blank content = %v, want title", blank.Content)
	}
}

func TestRandomEventColorDoesNotRepeatImmediately(t *testing.T) {
	previous := randomEventColor()
	for range 64 {
		next := randomEventColor()
		if next == previous {
			t.Fatalf("random color repeated immediately: %s", next)
		}
		previous = next
	}
}

func TestTitleIsUniqueAndContentCanRepeat(t *testing.T) {
	service := testService(t)
	sharedContent := "相同内容"

	createTestTodo(t, service, "标题一", &sharedContent)
	createTestTodo(t, service, "标题二", &sharedContent)

	_, err := service.Create(context.Background(), CreateCommand{
		Title:      "标题一",
		Content:    stringPointer("不同内容"),
		StartsAt:   func() *time.Time { value := time.Now(); return &value }(),
		RepeatMode: model.RepeatOnce,
	})
	if !errors.Is(err, ErrTitleConflict) {
		t.Fatalf("duplicate title error = %v, want %v", err, ErrTitleConflict)
	}
}

func TestPatchBlankContentUsesEffectiveTitle(t *testing.T) {
	service := testService(t)
	item := createTestTodo(t, service, "旧标题", stringPointer("旧内容"))
	blank := "  "

	updated, err := service.Patch(context.Background(), item.ID, PatchCommand{
		Content: &blank,
		Version: item.Version,
	})
	if err != nil {
		t.Fatalf("patch blank content with old title: %v", err)
	}
	if updated.Content == nil || *updated.Content != "旧标题" {
		t.Fatalf("content = %v, want old title", updated.Content)
	}

	newTitle := "新标题"
	updated, err = service.Patch(context.Background(), item.ID, PatchCommand{
		Title:   &newTitle,
		Content: &blank,
		Version: updated.Version,
	})
	if err != nil {
		t.Fatalf("patch blank content with new title: %v", err)
	}
	if updated.Title != newTitle || updated.Content == nil || *updated.Content != newTitle {
		t.Fatalf("updated todo = %#v", updated)
	}
}
