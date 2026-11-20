package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"note/internal/calendarsync"
	"note/internal/event"
	"note/internal/model"
	"note/internal/todo"

	"gorm.io/gorm"
)

// Each operation must commit its data and recipient tasks together, or neither.
func TestCalendarBusinessTransactions(t *testing.T) {
	for _, kind := range []string{"todo", "event"} {
		ops := []string{"create", "update", "delete", "role"}
		if kind == "todo" {
			ops = append(ops, "complete", "uncomplete")
		}
		for _, op := range ops {
			for _, fail := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/fail=%v", kind, op, fail), func(t *testing.T) {
					db, owner, member, outsider, group, _ := groupManagementFixture(t)
					// Three current members; role changes need only the actor and target.
					if err := db.Create(&model.GroupMember{GroupID: group.ID, UserID: outsider.ID}).Error; err != nil {
						t.Fatal(err)
					}
					ctx := context.Background()
					tr, er := todo.NewGORMRepository(db), event.NewGORMRepository(db)
					start := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC)
					content, title := "content", "changed"
					td := model.Todo{GroupID: group.ID, CreatorID: owner.ID, Title: "old", Content: &content, StartsAt: &start}
					ev := model.Event{GroupID: group.ID, CreatorID: owner.ID, Title: "old", StartsAt: start, EndsAt: start.Add(time.Hour)}
					create := func() error {
						if kind == "todo" {
							return tr.Create(ctx, &td)
						}
						return er.Create(ctx, &ev)
					}
					if op != "create" {
						if err := create(); err != nil {
							t.Fatal(err)
						}
					}
					if op == "uncomplete" {
						if err := tr.SetOccurrenceDone(ctx, td.ID, member.ID, start, true); err != nil {
							t.Fatal(err)
						}
					}
					if err := db.Where("name = ?", "cldr").Delete(&model.Outbox{}).Error; err != nil {
						t.Fatal(err)
					}
					before := calendarDBState(t, db)
					if fail {
						// Fail on the second recipient to also verify partial inserts roll back.
						if err := db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_calendar BEFORE INSERT ON outboxes
						WHEN NEW.name = 'cldr' AND NEW.receiver_id = %d
						BEGIN SELECT RAISE(ABORT, 'calendar outbox failure'); END`, member.ID)).Error; err != nil {
							t.Fatal(err)
						}
					}
					var err error
					if op == "create" {
						err = create()
					} else if kind == "todo" {
						switch op {
						case "update":
							_, err = tr.Patch(ctx, td.ID, owner.ID, todo.PatchCommand{Version: td.Version, Title: &title})
						case "delete":
							err = tr.Delete(ctx, td.ID, owner.ID)
						case "role":
							err = tr.PatchRole(ctx, owner.ID, td.ID, []uint{member.ID}, model.TodoEditor)
						case "complete", "uncomplete":
							err = tr.SetOccurrenceDone(ctx, td.ID, member.ID, start, op == "complete")
						}
					} else {
						switch op {
						case "update":
							ev.Title = title
							err = er.Update(ctx, &ev, ev.Version, owner.ID)
						case "delete":
							err = er.Delete(ctx, ev.ID, owner.ID)
						case "role":
							err = er.PatchRole(ctx, owner.ID, ev.ID, []uint{member.ID}, model.EventEditor)
						}
					}
					if fail {
						if err == nil {
							t.Fatal("expected outbox failure")
						}
						if after := calendarDBState(t, db); !reflect.DeepEqual(before, after) {
							t.Fatal("business or outbox changes survived rollback")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					var tasks []model.Outbox
					if err := db.Where("name = ?", "cldr").Order("receiver_id").Find(&tasks).Error; err != nil {
						t.Fatal(err)
					}
					want := []uint{owner.ID, member.ID, outsider.ID}
					if op == "role" {
						want = want[:2]
					}
					if len(tasks) != len(want) {
						t.Fatalf("tasks=%d want=%d", len(tasks), len(want))
					}
					id, action := td.ID, "upsert"
					if kind == "event" {
						id = ev.ID
					}
					if op == "delete" {
						action = "remove"
					}
					for i, task := range tasks {
						var change calendarsync.Change
						if err := json.Unmarshal([]byte(task.Data), &change); err != nil {
							t.Fatal(err)
						}
						if task.ReceiverID != want[i] || task.Revision != 0 || task.PublishedAt != nil ||
							change != (calendarsync.Change{GroupID: group.ID, Kind: kind, ID: id, Action: action}) {
							t.Fatalf("wrong task: %+v %+v", task, change)
						}
					}
					if op == "complete" || op == "uncomplete" {
						if err := tr.SetOccurrenceDone(ctx, td.ID, member.ID, start, op == "complete"); err != nil {
							t.Fatal(err)
						}
						var count int64
						if err := db.Model(&model.Outbox{}).Where("name = ?", "cldr").Count(&count).Error; err != nil {
							t.Fatal(err)
						}
						if count != int64(len(want)) {
							t.Fatal("unchanged completion created extra tasks")
						}
					}
				})
			}
		}
	}
}

func calendarDBState(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	state := make(map[string]string)
	for _, table := range []string{"todos", "todo_members", "todo_dates", "todo_completions", "events", "event_members", "event_dates", "outboxes", "notification_states"} {
		var rows []map[string]any
		if err := db.Table(table).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		state[table] = string(data)
	}
	return state
}
