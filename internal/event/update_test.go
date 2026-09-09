package event

import (
	"context"
	"errors"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	apperrors "note/internal/errors"
	"note/internal/model"
	"path/filepath"
	"testing"
	"time"
)

func TestPatchEventPersistsRangeAndRejectsStaleVersion(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(filepath.Join(t.TempDir(), "events.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.Event{}, &model.EventDate{}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewGORMRepository(db))
	start := time.Date(2026, 9, 9, 23, 0, 0, 0, time.FixedZone("CST", 8*3600))
	item, err := svc.Create(context.Background(), CreateCommand{Title: "跨天周期", StartsAt: start, EndsAt: start.Add(3 * time.Hour), RepeatMode: model.RepeatCustom, CustomDates: []time.Time{start}})
	if err != nil {
		t.Fatal(err)
	}
	end := start.Add(4 * time.Hour)
	updated, err := svc.Patch(context.Background(), item.ID, PatchCommand{EndsAt: &end, Version: item.Version})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := svc.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.EndsAt.Equal(end) || loaded.Version != item.Version+1 || len(loaded.CustomDates) != 1 || loaded.RepeatMode != model.RepeatCustom {
		t.Fatalf("unexpected saved event: %+v", loaded)
	}
	if _, err := svc.Patch(context.Background(), item.ID, PatchCommand{EndsAt: &end, Version: item.Version}); !errors.Is(err, apperrors.ErrEventConcurrentUpdate) {
		t.Fatalf("stale version error=%v", err)
	}
	invalidEnd := start.Add(-time.Minute)
	if _, err := svc.Patch(context.Background(), item.ID, PatchCommand{EndsAt: &invalidEnd, Version: updated.Version}); !errors.Is(err, apperrors.ErrInvalidEventTimeRange) {
		t.Fatalf("invalid range error=%v", err)
	}
	after, _ := svc.Get(context.Background(), item.ID)
	if !after.EndsAt.Equal(end) || after.Version != updated.Version {
		t.Fatal("rejected edit changed stored event")
	}
	if _, err := svc.Get(context.Background(), 9999); !errors.Is(err, apperrors.ErrEventNotFound) {
		t.Fatalf("missing event error=%v", err)
	}
}
