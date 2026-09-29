package internal

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"note/internal/auth"
	apperrors "note/internal/errors"
	"note/internal/model"
)

func TestAuthConcurrentAccountCollision(t *testing.T) {
	db := authTestDB(t)
	// 使用多个连接模拟竞争，唯一性不能只依靠单连接排队。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	repo := auth.NewGORMRepository(db)
	start := make(chan struct{})
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			<-start
			results <- repo.Create(context.Background(), &model.User{
				Username: "小明", Suffix: 12345, Email: fmt.Sprintf("user%d@example.com", i),
				PasswordHash: "test-hash", Nickname: "小明",
			})
		}(i)
	}
	close(start)
	successes := 0
	for i := 0; i < 8; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, apperrors.ErrAccountTaken) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("got %d winners, want 1", successes)
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
	var winner model.User
	if err := db.First(&winner).Error; err != nil {
		t.Fatal(err)
	}
	if winner.PasswordHash != "test-hash" {
		t.Fatal("existing user modified")
	}
	err = repo.Create(context.Background(), &model.User{Username: "小红", Suffix: 54321, Email: winner.Email, PasswordHash: "different", Nickname: "小红"})
	if !errors.Is(err, apperrors.ErrEmailTaken) {
		t.Fatalf("email conflict: %v", err)
	}
}

func TestAuthConcurrentSameNameRegistration(t *testing.T) {
	db := authTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(auth.NewGORMRepository(db))
	type result struct {
		user *model.User
		err  error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			<-start
			user, err := service.Register(context.Background(), "小明", fmt.Sprintf("user%d@example.com", i), "Demo_12345")
			results <- result{user, err}
		}(i)
	}
	close(start)
	suffixes := map[int]bool{}
	for i := 0; i < 8; i++ {
		result := <-results
		if result.err != nil {
			t.Errorf("register: %v", result.err)
			continue
		}
		if suffixes[result.user.Suffix] {
			t.Error("duplicate suffix for same name")
		}
		suffixes[result.user.Suffix] = true
	}
	if len(suffixes) != 8 {
		t.Fatalf("got %d unique accounts, want 8", len(suffixes))
	}
}
