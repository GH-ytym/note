package search

import (
	"context"
	"note/internal/model"
	"path/filepath"
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestSearchAll(t *testing.T) {
	//1.临时数据库
	dbPath := filepath.Join(t.TempDir(), "search.db")
	//db是gorm连接对象，用于执行数据库操作
	db, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("数据库连接失败:%v", err)
	}
	//2.获取数据库连接
	//sqldb是数据库连接池，用于存储数据库连接，避免频繁创建和销毁连接
	sqldb, err := db.DB()
	if err != nil {
		t.Fatalf("sqldb 获取失败:%v", err)
	}
	sqldb.SetMaxOpenConns(1)
	//延时关闭数据库连接
	//当前测试及其子测试全部结束后才会执行
	t.Cleanup(func() {
		_ = sqldb.Close()
	})
	//3.迁移表
	if err := db.AutoMigrate(&model.Todo{}, &model.Event{}); err != nil {
		t.Fatalf("数据库迁移失败:%v", err)
	}

	//4.构造测试数据
	start := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	content := "test content"

	todos := []model.Todo{
		{
			Title:      "开会",
			Content:    &content,
			StartsAt:   &start,
			RepeatMode: model.RepeatOnce,
			NotifyMode: model.NotifyNone,
		},
		{
			Title:      "下午开会",
			Content:    &content,
			StartsAt:   &start,
			RepeatMode: model.RepeatOnce,
			NotifyMode: model.NotifyNone,
		},
		{
			Title:      "买菜",
			Content:    &content,
			StartsAt:   &start,
			RepeatMode: model.RepeatOnce,
			NotifyMode: model.NotifyNone,
		},
	}
	if err := db.Create(&todos).Error; err != nil {
		t.Fatalf("insert todos: %v", err)
	}

	event := model.Event{
		Title:      "开会准备",
		Content:    &content,
		StartsAt:   start,
		EndsAt:     start.Add(time.Hour),
		RepeatMode: model.RepeatOnce,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("insert event: %v", err)
	}

	//创建搜索仓库
	repo := NewGORMRepository(db)
	//5.测试搜索
	items, total, err := repo.SearchAll(context.Background(), "开会", 1, 2)
	if err != nil {
		t.Fatalf("searchAll failed:%v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(items) != 2 {
		t.Fatalf("first page length = %d, want 2", len(items))
	}

	if items[0].Kind != "todo" ||
		items[0].ID != todos[0].ID ||
		items[0].Score != 100 {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Kind != "event" ||
		items[1].ID != event.ID ||
		items[1].Score != 80 {
		t.Fatalf("unexpected second item: %+v", items[1])
	}

	// 五、查询第二页，只有剩下的一条，但总数仍然是三条
	items, total, err = repo.SearchAll(
		context.Background(), "开会", 2, 2,
	)
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}
	if total != 3 || len(items) != 1 {
		t.Fatalf("second page: total=%d, length=%d", total, len(items))
	}
	if items[0].Kind != "todo" ||
		items[0].ID != todos[1].ID ||
		items[0].Score != 60 {
		t.Fatalf("unexpected second-page item: %+v", items[0])
	}
}

// Relevance must win over insertion order, and be applied before pagination.
func TestCategoryRanking(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(filepath.Join(t.TempDir(), "rank.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.Todo{}, &model.Event{}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	keyword := "100%_!"
	content := "正文包含 " + keyword
	// Exact, prefix, substring, content-only; IDs increase in reverse relevance.
	titles := []string{keyword, keyword + "计划", "完成" + keyword, "正文命中"}
	for i, title := range titles {
		updated := start.Add(time.Duration(i) * time.Hour)
		if err := db.Create(&model.Todo{Title: title, Content: &content, StartsAt: &start, UpdatedAt: updated}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.Event{Title: title, Content: &content, StartsAt: start, EndsAt: start.Add(time.Hour), UpdatedAt: updated}).Error; err != nil {
			t.Fatal(err)
		}
	}
	other := "100abc"
	if err := db.Create(&model.Todo{Title: other, Content: &other, StartsAt: &start}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Event{Title: other, Content: &other, StartsAt: start, EndsAt: start.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewGORMRepository(db)
	ctx := context.Background()
	for page := 1; page <= 4; page++ {
		wantScore := []int{100, 80, 60, 20}[page-1]
		todos, total, err := repo.SearchTodos(ctx, keyword, page, 1)
		if err != nil || total != 4 || len(todos) != 1 || todos[0].Title != titles[page-1] {
			t.Fatalf("todo page %d: %v, total %d, err %v", page, todos, total, err)
		}
		events, total, err := repo.SearchEvents(ctx, keyword, page, 1)
		if err != nil || total != 4 || len(events) != 1 || events[0].Title != titles[page-1] {
			t.Fatalf("event page %d: %v, total %d, err %v", page, events, total, err)
		}
		if todos[0].Score != wantScore || events[0].Score != wantScore {
			t.Fatalf("category scores must match merged search: todos=%d events=%d want=%d", todos[0].Score, events[0].Score, wantScore)
		}
	}
	all, total, err := repo.SearchAll(ctx, keyword, 1, 100)
	if err != nil || total != 8 {
		t.Fatalf("all total %d: %v", total, err)
	}
	for i, item := range all {
		if item.Title != titles[i/2] {
			t.Fatalf("merged order disagrees at %d: %+v", i, item)
		}
	}
}
