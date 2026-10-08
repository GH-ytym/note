package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/calendar"
	apperrors "note/internal/errors"
	"note/internal/event"
	"note/internal/group"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"
	"note/internal/todo"

	"gorm.io/gorm"
)

func eventGroupRequest(t *testing.T, db *gorm.DB) func(string, string, uint, string) *httptest.ResponseRecorder {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	es := event.NewService(event.NewGORMRepository(db))
	ts := todo.NewService(todo.NewGORMRepository(db))
	r := router.NewWithWeb(handler.NewTodoHandler(ts), handler.NewEventHandler(es),
		handler.NewCalendarHandler(calendar.NewService(ts, es)), nil,
		nil, handler.NewGroupHandler(group.NewService(group.NewGORMRepository(db))), nil, tokens, "")
	return func(method, path string, userID uint, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if userID != 0 {
			token, err := tokens.Generate(userID)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		return res
	}
}

func seedGroupEvent(t *testing.T, db *gorm.DB, groupID, creatorID uint, title string) model.Event {
	t.Helper()
	start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	item, err := event.NewService(event.NewGORMRepository(db)).Create(context.Background(), event.CreateCommand{
		GroupID: groupID, CreatorID: creatorID, Title: title, StartsAt: start, EndsAt: start.Add(time.Hour),
		RepeatMode: model.RepeatCustom, CustomDates: []time.Time{start},
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func checkEventRole(t *testing.T, db *gorm.DB, eventID, userID uint, want model.EventRole) {
	t.Helper()
	var member model.EventMember
	if err := db.Where("event_id = ? AND user_id = ?", eventID, userID).First(&member).Error; err != nil || member.Role != want {
		t.Fatalf("event %d user %d: role=%s want=%s err=%v", eventID, userID, member.Role, want, err)
	}
}

func TestEventGroupHTTPPermissionsAndIsolation(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	request := eventGroupRequest(t, db)
	body := fmt.Sprintf(`{"group_id":%d,"creator_id":%d,"title":"本群计划","starts_at":"2026-10-04T10:00:00+08:00","ends_at":"2026-10-04T11:00:00+08:00","repeat_mode":"custom","custom_dates":["2026-10-04"]}`, first.ID, owner.ID)
	for _, tc := range []struct {
		userID uint
		body   string
		status int
	}{
		{0, body, http.StatusUnauthorized}, {outsider.ID, body, http.StatusForbidden},
		{member.ID, strings.Replace(body, fmt.Sprintf(`"group_id":%d,`, first.ID), "", 1), http.StatusBadRequest},
	} {
		res := request(http.MethodPost, "/api/events", tc.userID, tc.body)
		if res.Code != tc.status {
			t.Fatalf("create: %d want=%d %s", res.Code, tc.status, res.Body.String())
		}
	}
	res := request(http.MethodPost, "/api/events", member.ID, body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var created handler.EventDetailResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.GroupID != first.ID || created.CreatorID != member.ID || created.Creator == nil || created.Creator.ID != member.ID || created.Creator.Avatar != "member-avatar" {
		t.Fatalf("JWT identity or creator profile lost: %+v", created)
	}
	if strings.Contains(res.Body.String(), "test-hash") || strings.Contains(res.Body.String(), member.Email) {
		t.Fatal("response exposed private user fields")
	}
	checkEventRole(t, db, created.ID, member.ID, model.EventEditor)
	checkEventRole(t, db, created.ID, owner.ID, model.EventViewer)
	other := seedGroupEvent(t, db, second.ID, owner.ID, "其他群计划")
	path := fmt.Sprintf("/api/events/%d", created.ID)
	roles := path + "/members"
	patch := `{"title":"调整后的计划","ends_at":"2026-10-05T01:00:00+08:00","version":1}`
	// 即使有残留 editor 授权，群外用户也不能读取或修改。
	if err := db.Create(&model.EventMember{EventID: created.ID, UserID: outsider.ID, Role: model.EventEditor}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path string
		userID       uint
		body         string
		status       int
	}{
		{http.MethodGet, path, owner.ID, "", 200},
		{http.MethodPatch, path, owner.ID, patch, 403},
		{http.MethodDelete, path, owner.ID, "", 403},
		{http.MethodPatch, roles, owner.ID, fmt.Sprintf(`{"user_ids":[%d],"role":1}`, owner.ID), 403},
		{http.MethodGet, path, outsider.ID, "", 403},
		{http.MethodPatch, path, outsider.ID, patch, 403},
		{http.MethodDelete, path, outsider.ID, "", 403},
		{http.MethodPatch, roles, outsider.ID, fmt.Sprintf(`{"user_ids":[%d],"role":1}`, owner.ID), 403},
		{http.MethodGet, fmt.Sprintf("/api/groups/%d/events", first.ID), outsider.ID, "", 403},
		{http.MethodGet, "/api/events/999999", member.ID, "", 404},
		{http.MethodPatch, roles, member.ID, fmt.Sprintf(`{"user_ids":[%d],"role":2}`, member.ID), 400},
		{http.MethodPatch, roles, member.ID, fmt.Sprintf(`{"user_ids":[%d,%d],"role":1}`, owner.ID, outsider.ID), 404},
		{http.MethodPatch, roles, member.ID, `{"user_ids":[],"role":1}`, 400},
		{http.MethodPatch, roles, member.ID, `{"user_ids":[1],"role":3}`, 400},
	} {
		res := request(tc.method, tc.path, tc.userID, tc.body)
		if res.Code != tc.status {
			t.Fatalf("%s %s user=%d: %d want=%d %s", tc.method, tc.path, tc.userID, res.Code, tc.status, res.Body.String())
		}
	}
	checkEventRole(t, db, created.ID, owner.ID, model.EventViewer)
	res = request(http.MethodGet, fmt.Sprintf("/api/groups/%d/events?page_size=1", first.ID), member.ID, "")
	var page struct {
		Data  []handler.EventDetailResponse `json:"data"`
		Total int64                         `json:"total"`
		Page  int                           `json:"page"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 || page.Total != 1 || page.Page != 1 || len(page.Data) != 1 || page.Data[0].ID != created.ID || page.Data[0].Creator == nil {
		t.Fatalf("group list: %d %s", res.Code, res.Body.String())
	}
	res = request(http.MethodPatch, roles, member.ID, fmt.Sprintf(`{"user_ids":[%d,%d],"role":1}`, owner.ID, owner.ID))
	if res.Code != 204 {
		t.Fatalf("promote: %d %s", res.Code, res.Body.String())
	}
	checkEventRole(t, db, created.ID, owner.ID, model.EventEditor)
	res = request(http.MethodPatch, path, owner.ID, patch)
	if res.Code != 200 {
		t.Fatalf("editor patch: %d %s", res.Code, res.Body.String())
	}
	var updated model.Event
	if err := json.Unmarshal(res.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.GroupID != first.ID || updated.CreatorID != member.ID || updated.RepeatMode != model.RepeatCustom || len(updated.CustomDates) != 1 || updated.EndsAt.Sub(updated.StartsAt) != 15*time.Hour {
		t.Fatalf("patch changed existing event semantics: %+v", updated)
	}
	res = request(http.MethodPatch, path, owner.ID, patch)
	if res.Code != 409 {
		t.Fatalf("stale version: %d %s", res.Code, res.Body.String())
	}
	res = request(http.MethodPatch, path, owner.ID, `{"ends_at":"2026-10-04T09:00:00+08:00","version":2}`)
	if res.Code != 400 {
		t.Fatalf("invalid range: %d %s", res.Code, res.Body.String())
	}
	for _, method := range []string{http.MethodDelete, http.MethodPatch} {
		target, body := path, ""
		if method == http.MethodPatch {
			target, body = roles, fmt.Sprintf(`{"user_ids":[%d],"role":2}`, owner.ID)
		}
		res = request(method, target, owner.ID, body)
		if res.Code != 403 {
			t.Fatalf("editor management: %d %s", res.Code, res.Body.String())
		}
	}
	res = request(http.MethodPatch, roles, member.ID, fmt.Sprintf(`{"user_ids":[%d],"role":2}`, owner.ID))
	if res.Code != 204 {
		t.Fatalf("demote: %d %s", res.Code, res.Body.String())
	}
	res = request(http.MethodPatch, path, owner.ID, `{"title":"禁止修改","version":2}`)
	if res.Code != 403 {
		t.Fatalf("demoted editor: %d %s", res.Code, res.Body.String())
	}
	// Calendar 按当前成员资格过滤。
	for _, userID := range []uint{member.ID, outsider.ID} {
		res = request(http.MethodGet, "/api/calendar?from=2026-10-04&to=2026-10-06", userID, "")
		var result struct {
			Data calendar.Result `json:"data"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		want := created.ID
		if userID == outsider.ID {
			want = other.ID
		}
		if res.Code != 200 || len(result.Data.Events) != 1 || result.Data.Events[0].EventID != want {
			t.Fatalf("calendar isolation: %d %s", res.Code, res.Body.String())
		}
	}
	res = request(http.MethodDelete, path, member.ID, "")
	if res.Code != 204 {
		t.Fatalf("creator delete: %d %s", res.Code, res.Body.String())
	}
	for _, table := range []any{&model.EventDate{}, &model.EventMember{}} {
		var count int64
		if err := db.Model(table).Where("event_id = ?", created.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("orphan event children: %T %d %v", table, count, err)
		}
	}
	if err := db.First(&model.Event{}, other.ID).Error; err != nil {
		t.Fatal("delete affected another group:", err)
	}
}

func TestEventGroupJoinQuitAndDismiss(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	es := event.NewService(event.NewGORMRepository(db))
	gs := group.NewGORMRepository(db)
	ctx := context.Background()
	item := seedGroupEvent(t, db, first.ID, member.ID, "创建者的计划")
	owned := seedGroupEvent(t, db, first.ID, owner.ID, "群主自己的计划")
	retained := seedGroupEvent(t, db, second.ID, owner.ID, "保留的计划")
	if err := gs.Quit(ctx, first.ID, member.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := es.Get(ctx, item.ID, member.ID); !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("former creator read: %v", err)
	}
	var permissions int64
	if err := db.Model(&model.EventMember{}).Where("event_id = ? AND user_id = ?", item.ID, member.ID).Count(&permissions).Error; err != nil || permissions != 0 {
		t.Fatalf("departing roles: %d %v", permissions, err)
	}
	loaded, err := es.Get(ctx, item.ID, owner.ID)
	if err != nil || len(loaded.CustomDates) != 1 || loaded.CreatorID != member.ID {
		t.Fatalf("quit lost event: %+v %v", loaded, err)
	}
	// 本群只有 Event，没有 Todo，也必须初始化 Event 权限。
	if _, _, err := gs.Join(ctx, outsider.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, outsider.ID, model.EventViewer)
	checkEventRole(t, db, owned.ID, outsider.ID, model.EventViewer)
	if _, _, err := gs.Join(ctx, member.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, member.ID, model.EventEditor)
	checkEventRole(t, db, owned.ID, member.ID, model.EventViewer)
	if err := es.PatchRole(ctx, member.ID, item.ID, []uint{outsider.ID}, model.EventEditor); err != nil {
		t.Fatal(err)
	}
	if _, _, err := gs.Join(ctx, outsider.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, outsider.ID, model.EventEditor)
	if err := gs.Quit(ctx, first.ID, outsider.ID, nil); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, retained.ID, outsider.ID, model.EventViewer)
	if _, _, err := gs.Join(ctx, outsider.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, outsider.ID, model.EventViewer)
	// 群主转让并退群，自己的 Event 保留；重新入群恢复创建者权限。
	if err := gs.Quit(ctx, first.ID, owner.ID, &member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := es.Get(ctx, owned.ID, owner.ID); !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("former owner read: %v", err)
	}
	if _, _, err := gs.Join(ctx, owner.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, owned.ID, owner.ID, model.EventEditor)
	checkEventRole(t, db, item.ID, owner.ID, model.EventViewer)
	if err := gs.Dismiss(ctx, first.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []any{&model.Event{}, &model.EventDate{}, &model.EventMember{}} {
		var count int64
		want := int64(1)
		if _, ok := table.(*model.EventMember); ok {
			want = 2
		}
		if err := db.Model(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("dismiss count %T: %d want=%d err=%v", table, count, want, err)
		}
	}
	loaded, err = es.Get(ctx, retained.ID, outsider.ID)
	if err != nil || loaded.GroupID != second.ID || len(loaded.CustomDates) != 1 {
		t.Fatalf("dismiss changed other group: %+v %v", loaded, err)
	}
}

func TestEventCreateAndRoleUpdatesRollBack(t *testing.T) {
	db, owner, member, outsider, first, _ := groupManagementFixture(t)
	es := event.NewService(event.NewGORMRepository(db))
	ctx := context.Background()
	if _, _, err := group.NewGORMRepository(db).Join(ctx, outsider.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	trigger := fmt.Sprintf(`CREATE TEMP TRIGGER reject_event_create_role
		BEFORE INSERT ON event_members WHEN NEW.user_id = %d
		BEGIN SELECT RAISE(ABORT, 'test event permission insert failure'); END`, member.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	if _, err := es.Create(ctx, event.CreateCommand{GroupID: first.ID, CreatorID: owner.ID,
		Title: "不应留下记录", StartsAt: start, EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatCustom,
		CustomDates: []time.Time{start}}); err == nil {
		t.Fatal("create succeeded despite failed permission insert")
	}
	for _, table := range []any{&model.Event{}, &model.EventDate{}, &model.EventMember{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("partial create %T: %d %v", table, count, err)
		}
	}
	if err := db.Exec("DROP TRIGGER reject_event_create_role").Error; err != nil {
		t.Fatal(err)
	}
	item := seedGroupEvent(t, db, first.ID, owner.ID, "批量授权")
	trigger = fmt.Sprintf(`CREATE TEMP TRIGGER reject_event_role_update
		AFTER UPDATE ON event_members WHEN NEW.user_id = %d
		BEGIN SELECT RAISE(FAIL, 'test event role update failure'); END`, outsider.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if err := es.PatchRole(ctx, owner.ID, item.ID, []uint{member.ID, outsider.ID}, model.EventEditor); err == nil {
		t.Fatal("role update succeeded despite trigger failure")
	}
	checkEventRole(t, db, item.ID, member.ID, model.EventViewer)
	checkEventRole(t, db, item.ID, outsider.ID, model.EventViewer)
	if err := db.Exec("DROP TRIGGER reject_event_role_update").Error; err != nil {
		t.Fatal(err)
	}
	if err := es.PatchRole(ctx, owner.ID, item.ID, []uint{member.ID, outsider.ID}, model.EventEditor); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, member.ID, model.EventEditor)
	checkEventRole(t, db, item.ID, outsider.ID, model.EventEditor)
}

func TestEventGroupJoinAndDismissRollBack(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	ctx := context.Background()
	gs := group.NewGORMRepository(db)
	item := seedGroupEvent(t, db, first.ID, owner.ID, "本群计划")
	retained := seedGroupEvent(t, db, second.ID, owner.ID, "其他群计划")
	todoItem := seedGroupManagementTodo(t, db, first.ID, owner.ID, member.ID)
	trigger := fmt.Sprintf(`CREATE TEMP TRIGGER reject_join_event_role
		BEFORE INSERT ON event_members WHEN NEW.user_id = %d AND NEW.event_id = %d
		BEGIN SELECT RAISE(ABORT, 'test join event permission failure'); END`, outsider.ID, item.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := gs.Join(ctx, outsider.ID, "ABC123"); err == nil {
		t.Fatal("join succeeded despite failed event role insert")
	}
	var count int64
	if err := db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", first.ID, outsider.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed join kept membership: %d %v", count, err)
	}
	if err := db.Model(&model.TodoMember{}).Where("todo_id = ? AND user_id = ?", todoItem.ID, outsider.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed join kept todo role: %d %v", count, err)
	}
	checkEventRole(t, db, retained.ID, outsider.ID, model.EventViewer)
	if err := db.Exec("DROP TRIGGER reject_join_event_role").Error; err != nil {
		t.Fatal(err)
	}
	trigger = fmt.Sprintf(`CREATE TEMP TRIGGER reject_dismiss_with_events
		BEFORE DELETE ON groups WHEN OLD.id = %d
		BEGIN SELECT RAISE(ABORT, 'test event dismissal failure'); END`, first.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if err := gs.Dismiss(ctx, first.ID, owner.ID); err == nil {
		t.Fatal("dismiss succeeded despite failed group deletion")
	}
	var loaded model.Event
	if err := db.Preload("CustomDates").Preload("Members").First(&loaded, item.ID).Error; err != nil || len(loaded.CustomDates) != 1 || len(loaded.Members) != 2 {
		t.Fatalf("dismiss lost event data: %+v %v", loaded, err)
	}
	for _, tc := range []struct {
		table any
		count int64
	}{
		{&model.Group{}, 2}, {&model.GroupMember{}, 4}, {&model.Todo{}, 1},
		{&model.TodoMember{}, 2}, {&model.TodoDate{}, 1}, {&model.TodoCompletion{}, 1},
		{&model.Event{}, 2}, {&model.EventDate{}, 2}, {&model.EventMember{}, 4},
	} {
		if err := db.Model(tc.table).Count(&count).Error; err != nil || count != tc.count {
			t.Fatalf("dismiss rollback %T: %d want=%d %v", tc.table, count, tc.count, err)
		}
	}
}

func TestEventUpdateRechecksMembershipAndRole(t *testing.T) {
	db, owner, member, _, first, _ := groupManagementFixture(t)
	ctx := context.Background()
	repo := event.NewGORMRepository(db)
	es := event.NewService(repo)
	item := seedGroupEvent(t, db, first.ID, member.ID, "原始标题")
	if err := es.PatchRole(ctx, member.ID, item.ID, []uint{owner.ID}, model.EventEditor); err != nil {
		t.Fatal(err)
	}
	previous, err := repo.Get(ctx, item.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	previous.Title = "不应保存"
	// 模拟 Service 读完后，创建者立刻把操作者降为 viewer。
	if err := es.PatchRole(ctx, member.ID, item.ID, []uint{owner.ID}, model.EventViewer); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, &previous, item.Version, owner.ID); !errors.Is(err, apperrors.ErrEventEditDenied) {
		t.Fatalf("revoked editor update: %v", err)
	}
	if err := es.PatchRole(ctx, member.ID, item.ID, []uint{owner.ID}, model.EventEditor); err != nil {
		t.Fatal(err)
	}
	if err := group.NewGORMRepository(db).Quit(ctx, first.ID, owner.ID, &member.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, &previous, item.Version, owner.ID); !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("departed editor update: %v", err)
	}
	loaded, err := es.Get(ctx, item.ID, member.ID)
	if err != nil || loaded.Title != item.Title || loaded.Version != item.Version {
		t.Fatalf("rejected update changed event: %+v %v", loaded, err)
	}
}

func TestEventConcurrentCreateJoinAndPatch(t *testing.T) {
	db, owner, _, outsider, first, _ := groupManagementFixture(t)
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(4)
	ctx := context.Background()
	es := event.NewService(event.NewGORMRepository(db))
	start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	ready := make(chan struct{})
	joined := make(chan error, 1)
	type createResult struct {
		item model.Event
		err  error
	}
	created := make(chan createResult, 1)
	go func() {
		<-ready
		_, _, err := group.NewGORMRepository(db).Join(ctx, outsider.ID, "ABC123")
		joined <- err
	}()
	go func() {
		<-ready
		item, err := es.Create(ctx, event.CreateCommand{GroupID: first.ID, CreatorID: owner.ID,
			Title: "并发创建", StartsAt: start, EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatOnce})
		created <- createResult{item, err}
	}()
	close(ready)
	if err := <-joined; err != nil {
		t.Fatal(err)
	}
	result := <-created
	if result.err != nil {
		t.Fatal(result.err)
	}
	checkEventRole(t, db, result.item.ID, outsider.ID, model.EventViewer)
	// 同一版本同时提交两份修改，只能成功一份。
	ready = make(chan struct{})
	patched := make(chan error, 2)
	for _, title := range []string{"修改 A", "修改 B"} {
		go func(title string) {
			<-ready
			_, err := es.Patch(ctx, result.item.ID, owner.ID, event.PatchCommand{Title: &title, Version: result.item.Version})
			patched <- err
		}(title)
	}
	close(ready)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-patched
		if err == nil {
			successes++
		} else if errors.Is(err, apperrors.ErrEventConcurrentUpdate) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
}

func TestEventReadOnlyQueriesAndMigration(t *testing.T) {
	db, owner, member, _, first, _ := groupManagementFixture(t)
	item := seedGroupEvent(t, db, first.ID, owner.ID, "已提交标题")
	es := event.NewService(event.NewGORMRepository(db))
	if err := es.PatchRole(context.Background(), owner.ID, item.ID, []uint{member.ID}, model.EventEditor); err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	checkEventRole(t, db, item.ID, member.ID, model.EventEditor)
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(4)
	writer, err := pool.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.Exec("UPDATE events SET title = ? WHERE id = ?", "未提交标题", item.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	loaded, err := es.Get(ctx, item.ID, member.ID)
	if err != nil || loaded.Title != item.Title || len(loaded.CustomDates) != 1 {
		t.Fatalf("read while writing: %+v %v", loaded, err)
	}
	page, err := es.List(ctx, event.ListQuery{GroupID: first.ID, UserID: member.ID})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Title != item.Title {
		t.Fatalf("list while writing: %+v %v", page, err)
	}
	from := time.Date(2026, 10, 4, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	occurrences, err := es.ListInRange(ctx, member.ID, from, from.AddDate(0, 0, 1))
	if err != nil || len(occurrences) != 1 || occurrences[0].Title != item.Title {
		t.Fatalf("calendar while writing: %+v %v", occurrences, err)
	}
}
