package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/calendar"
	"note/internal/event"
	"note/internal/handler"
	"note/internal/middleware"
	"note/internal/model"
	"note/internal/todo"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestCalendarGroupFilterAtDatabase(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	ctx := context.Background()
	tr, er := todo.NewGORMRepository(db), event.NewGORMRepository(db)
	start := time.Date(2026, 11, 20, 9, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	content := "content"
	for _, group := range []model.Group{first, second} {
		td := model.Todo{GroupID: group.ID, CreatorID: owner.ID, Title: group.Name, Content: &content, StartsAt: &start, RepeatMode: model.RepeatDaily}
		if err := tr.Create(ctx, &td); err != nil {
			t.Fatal(err)
		}
		if err := tr.SetOccurrenceDone(ctx, td.ID, owner.ID, start, true); err != nil {
			t.Fatal(err)
		}
		ev := model.Event{GroupID: group.ID, CreatorID: owner.ID, Title: group.Name, StartsAt: start, EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatCustom,
			CustomDates: []model.EventDate{{Date: start}}}
		if err := er.Create(ctx, &ev); err != nil {
			t.Fatal(err)
		}
	}
	svc := calendar.NewService(todo.NewService(tr), event.NewService(er))
	var groupID uint
	rows := map[string]int64{}
	// Check what SQL actually loads, not just the final HTTP response.
	if err := db.Callback().Query().After("gorm:query").Register("calendar_filter_probe", func(tx *gorm.DB) {
		table := tx.Statement.Table
		if table == "todos" || table == "events" || table == "todo_completions" {
			rows[table] += tx.RowsAffected
		}
		if groupID != 0 && (table == "todos" || table == "events") &&
			!strings.Contains(tx.Statement.SQL.String(), table+".group_id =") {
			t.Errorf("group filter missing from SQL: %s", tx.Statement.SQL.String())
		}
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		uid, group uint
		groups     int
	}{
		{"owner first", owner.ID, first.ID, 1},
		{"owner second", owner.ID, second.ID, 1},
		{"all joined", owner.ID, 0, 2},
		{"viewer", member.ID, first.ID, 1},
		{"foreign group", member.ID, second.ID, 0},
		{"outsider", outsider.ID, first.ID, 0},
		{"nonexistent", owner.ID, 999999, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID = tc.group
			clear(rows)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Set(middleware.UserIDKey, tc.uid)
			c.Request = httptest.NewRequest("GET", fmt.Sprintf("/calendar?from=2026-11-20&to=2026-11-22&group_id=%d&user_id=%d", tc.group, owner.ID), nil)
			handler.NewCalendarHandler(svc).GetCalendar(c)
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var result struct {
				Data calendar.Result `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Data.Todos) != tc.groups*2 || len(result.Data.Events) != tc.groups {
				t.Fatalf("unexpected occurrences: %+v", result.Data)
			}
			for _, item := range result.Data.Todos {
				if tc.group != 0 && item.GroupID != tc.group {
					t.Fatal("todo leaked across groups")
				}
			}
			for _, item := range result.Data.Events {
				if tc.group != 0 && item.GroupID != tc.group {
					t.Fatal("event leaked across groups")
				}
			}
			for _, table := range []string{"todos", "events", "todo_completions"} {
				if rows[table] != int64(tc.groups) {
					t.Fatalf("%s fetched %d rows, want %d", table, rows[table], tc.groups)
				}
			}
		})
	}
	// Old per-record permissions do not grant access after leaving the group.
	if err := db.Where("group_id = ? AND user_id = ?", first.ID, member.ID).Delete(&model.GroupMember{}).Error; err != nil {
		t.Fatal(err)
	}
	groupID = first.ID
	result, err := svc.Get(ctx, member.ID, first.ID, start, start.AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Todos) != 0 || len(result.Events) != 0 {
		t.Fatal("departed member retained calendar access")
	}
}
