package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"note/internal/calendar"
	"note/internal/event"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/todo"
	"testing"
)

func TestCalendarAndDetailReflectCurrentMemberRole(t *testing.T) {
	db, owner, creator, group := groupSchemaFixture(t)
	if err := db.Create(&[]model.GroupMember{{GroupID: group.ID, UserID: owner.ID}, {GroupID: group.ID, UserID: creator.ID}}).Error; err != nil {
		t.Fatal(err)
	}
	ts := todo.NewService(todo.NewGORMRepository(db))
	es := event.NewService(event.NewGORMRepository(db))
	item := createTestTodo(t, ts, group.ID, creator.ID)
	eventItem := seedGroupEvent(t, db, group.ID, creator.ID, "小组讨论")
	request := eventGroupRequest(t, db)
	check := func(actor uint, role string, roleCount int) {
		t.Helper()
		response := request("GET", fmt.Sprintf("/api/calendar?from=2026-10-04&to=2026-10-05&group_id=%d", group.ID), actor, "")
		if response.Code != 200 {
			t.Fatalf("calendar: %d %s", response.Code, response.Body.String())
		}
		var result struct {
			Data calendar.Result `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data.Todos) != 1 || len(result.Data.Events) != 1 {
			t.Fatalf("unexpected calendar: %+v", result)
		}
		if string(result.Data.Todos[0].MyRole) != role || string(result.Data.Events[0].MyRole) != role || result.Data.Todos[0].GroupID != group.ID || result.Data.Todos[0].CreatorID != creator.ID {
			t.Fatalf("actor %d: %+v", actor, result)
		}
		response = request("GET", fmt.Sprintf("/api/todos/%d", item.ID), actor, "")
		var todoResult handler.TodoDetailResponse
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &todoResult); err != nil {
			t.Fatal(err)
		}
		response = request("GET", fmt.Sprintf("/api/events/%d", eventItem.ID), actor, "")
		var eventResult handler.EventDetailResponse
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &eventResult); err != nil {
			t.Fatal(err)
		}
		if string(todoResult.MyRole) != role || string(eventResult.MyRole) != role || len(todoResult.MemberRoles) != roleCount || len(eventResult.MemberRoles) != roleCount {
			t.Fatalf("detail role for actor %d: %s, %s", actor, todoResult.MyRole, eventResult.MyRole)
		}
	}
	// 群主对他人创建的内容默认也是 viewer；创建者拥有 editor 和权限列表。
	check(owner.ID, "viewer", 0)
	check(creator.ID, "editor", 2)
	for _, role := range []string{"editor", "viewer"} {
		if err := ts.PatchRole(context.Background(), creator.ID, item.ID, []uint{owner.ID}, model.TodoRole(role)); err != nil {
			t.Fatal(err)
		}
		if err := es.PatchRole(context.Background(), creator.ID, eventItem.ID, []uint{owner.ID}, model.EventRole(role)); err != nil {
			t.Fatal(err)
		}
		check(owner.ID, role, 0)
	}
}
