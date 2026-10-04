package internal

import (
	"context"
	"strings"
	"testing"
	"time"

	"note/internal/event"
	"note/internal/model"
)

func TestMigrateEmptyLegacyEventSchema(t *testing.T) {
	db := authTestDB(t)
	if err := db.Exec(`CREATE TABLE events (
		id integer PRIMARY KEY AUTOINCREMENT,
		title text NOT NULL, content text, color text NOT NULL DEFAULT '#F3B51B',
		starts_at datetime NOT NULL, ends_at datetime NOT NULL,
		repeat_mode text NOT NULL DEFAULT 'once', created_at datetime, updated_at datetime,
		version integer NOT NULL DEFAULT 1,
		CONSTRAINT chk_events_time_range CHECK (ends_at > starts_at)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.EventMember{}) || !db.Migrator().HasColumn("events", "group_id") || !db.Migrator().HasColumn("events", "creator_id") {
		t.Fatal("legacy empty event schema incomplete")
	}
	owner := model.User{Username: "event_owner", Suffix: 12345, Email: "event-owner@example.com", PasswordHash: "test-hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	group := model.Group{Name: "日程群", OwnerID: owner.ID, Code: "ABC123"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GroupMember{GroupID: group.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	item, err := event.NewService(event.NewGORMRepository(db)).Create(context.Background(), event.CreateCommand{
		GroupID: group.ID, CreatorID: owner.ID, Title: "迁移后的日程", StartsAt: start,
		EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatOnce,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, owner.ID, model.EventEditor)
	var tableSQL string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'events'").Scan(&tableSQL).Error; err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"REFERENCES groups(id)", "REFERENCES users(id)", "chk_events_time_range"} {
		if !strings.Contains(tableSQL, fragment) {
			t.Fatalf("missing constraint %s: %s", fragment, tableSQL)
		}
	}
}
