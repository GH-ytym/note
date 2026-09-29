package internal

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/todo"
)

func TestTodoCompletionDeleteRace(t *testing.T) {
	db := authTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	repo := todo.NewGORMRepository(db)
	service := todo.NewService(repo)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	date := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		content := "race"
		item := model.Todo{Title: fmt.Sprintf("race%d", i), Content: &content, StartsAt: &date}
		if err := repo.Create(ctx, &item); err != nil {
			t.Fatal(err)
		}
		// 重复完成和重复取消均幂等。
		for _, done := range []bool{true, true, false, false} {
			if err := service.SetOccurrenceDone(ctx, item.ID, date, done); err != nil {
				t.Fatal(err)
			}
		}
		start := make(chan struct{})
		completion := make(chan error, 1)
		deletion := make(chan error, 1)
		go func() { <-start; completion <- service.SetOccurrenceDone(ctx, item.ID, date, true) }()
		go func() { <-start; deletion <- service.Delete(ctx, item.ID) }()
		close(start)
		if err := <-completion; err != nil && !errors.Is(err, apperrors.ErrTodoNotFound) {
			t.Fatalf("completion: %v", err)
		}
		if err := <-deletion; err != nil {
			t.Fatalf("delete: %v", err)
		}
		var count int64
		if err := db.Model(&model.TodoCompletion{}).Where("todo_id = ?", item.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("orphan count=%d err=%v", count, err)
		}
		for _, done := range []bool{true, false} {
			if err := service.SetOccurrenceDone(ctx, item.ID, date, done); !errors.Is(err, apperrors.ErrTodoNotFound) {
				t.Fatalf("deleted todo: %v", err)
			}
		}
	}
}

func TestTodoListConcurrentWrites(t *testing.T) {
	db := authTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	repo := todo.NewGORMRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := make(chan struct{})
	writes := make(chan error, 1)
	go func() {
		<-start
		date := time.Now()
		for i := 0; i < 50; i++ {
			content := "list"
			item := model.Todo{Title: fmt.Sprintf("list%d", i), Content: &content, StartsAt: &date}
			if err := repo.Create(ctx, &item); err != nil {
				writes <- err
				return
			}
			if i%2 == 0 {
				if err := repo.Delete(ctx, item.ID); err != nil {
					writes <- err
					return
				}
			}
		}
		writes <- nil
	}()
	close(start)
	for i := 0; i < 100; i++ {
		// 总数据量小于页面大小，所以同一快照中 total 必须等于返回条数。
		items, total, err := repo.List(ctx, todo.ListQuery{Page: 1, PageSize: 100})
		if err != nil {
			t.Error(err)
			break
		}
		if total != int64(len(items)) {
			t.Errorf("mixed snapshot: total=%d len=%d", total, len(items))
			break
		}
	}
	if err := <-writes; err != nil {
		t.Fatal(err)
	}
}
