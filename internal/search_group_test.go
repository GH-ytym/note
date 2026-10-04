package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/event"
	"note/internal/group"
	"note/internal/model"
	"note/internal/router"
	"note/internal/search"
	"note/internal/todo"

	"gorm.io/gorm"
)

func groupSearchRequest(t *testing.T, db *gorm.DB) func(string, uint, int) search.Result {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	sh := search.NewSearchHandler(search.NewService(search.NewGORMRepository(db)))
	r := router.NewWithWeb(nil, nil, nil, sh, nil, nil, tokens, "")
	return func(path string, userID uint, want int) search.Result {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if userID != 0 {
			token, err := tokens.Generate(userID)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("user=%d path=%s: got %d want %d: %s", userID, path, res.Code, want, res.Body.String())
		}
		if res.Code != http.StatusOK {
			return search.Result{}
		}
		var body struct {
			Page search.Result `json:"page"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Page
	}
}

func TestSearchGroupHTTPIsolationAndMembership(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	start := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	content := "开会内容"
	var firstTodo model.Todo
	var firstEvent model.Event
	for _, groupID := range []uint{first.ID, second.ID} {
		for _, title := range []string{"开会", "仅正文命中"} {
			item := model.Todo{GroupID: groupID, CreatorID: owner.ID, Title: title, Content: &content, StartsAt: &start, RepeatMode: model.RepeatOnce}
			if err := db.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			event := model.Event{GroupID: groupID, CreatorID: owner.ID, Title: title, Content: &content, StartsAt: start, EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatOnce}
			if err := db.Create(&event).Error; err != nil {
				t.Fatal(err)
			}
			if groupID == first.ID && title == "开会" {
				firstTodo, firstEvent = item, event
			}
		}
	}
	// editor 授权不能让群外账号获得搜索访问权。
	for _, userID := range []uint{member.ID, outsider.ID} {
		if err := db.Create(&model.TodoMember{TodoID: firstTodo.ID, UserID: userID, Role: model.TodoEditor}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.EventMember{EventID: firstEvent.ID, UserID: userID, Role: model.EventEditor}).Error; err != nil {
			t.Fatal(err)
		}
	}
	request := groupSearchRequest(t, db)
	for _, kind := range []string{"todos", "events", "all"} {
		t.Run(kind, func(t *testing.T) {
			wantTotal := int64(2)
			if kind == "all" {
				wantTotal = 4
			}
			for _, prefix := range []string{"", "/api"} {
				path := fmt.Sprintf("%s/groups/%d/search/%s?keyword=开会&page_size=1", prefix, first.ID, kind)
				request(path, 0, http.StatusUnauthorized)
				request(path, outsider.ID, http.StatusForbidden)
				request(path+fmt.Sprintf("&user_id=%d&group_id=%d", owner.ID, second.ID), outsider.ID, http.StatusForbidden)
				for _, userID := range []uint{owner.ID, member.ID} {
					page := request(path, userID, http.StatusOK)
					if page.Total != wantTotal || page.Page != 1 || page.PageSize != 1 || len(page.Items) != 1 || page.Items[0].GroupID != first.ID || page.Items[0].Score != 100 {
						t.Fatalf("wrong group, total or ranking: %+v", page)
					}
				}
				page := request(path+fmt.Sprintf("&page=%d", wantTotal), member.ID, http.StatusOK)
				if page.Total != wantTotal || len(page.Items) != 1 || page.Items[0].GroupID != first.ID || page.Items[0].Score != 20 {
					t.Fatalf("content match escaped group filter or pagination: %+v", page)
				}
				page = request(path+fmt.Sprintf("&page=%d", wantTotal+1), member.ID, http.StatusOK)
				if page.Total != wantTotal || page.Items == nil || len(page.Items) != 0 {
					t.Fatalf("empty page lost group total or array shape: %+v", page)
				}
			}
		})
	}
	if err := db.Where("group_id = ? AND user_id = ?", first.ID, member.ID).Delete(&model.GroupMember{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"todos", "events", "all"} {
		path := fmt.Sprintf("/api/groups/%d/search/%s?keyword=开会", first.ID, kind)
		request(path, member.ID, http.StatusForbidden)
		// 群主仍是另一个群的成员，但必须显式选择那个群才能搜索它。
		other := request(fmt.Sprintf("/api/groups/%d/search/%s?keyword=开会", second.ID, kind), owner.ID, http.StatusOK)
		for _, item := range other.Items {
			if item.GroupID != second.ID {
				t.Fatalf("search crossed groups: %+v", other)
			}
		}
	}
}

func TestSearchGroupHTTPValidationAndLegacyRoutes(t *testing.T) {
	db, owner, _, outsider, first, _ := groupManagementFixture(t)
	request := groupSearchRequest(t, db)
	for _, kind := range []string{"todos", "events", "all"} {
		for _, prefix := range []string{"", "/api"} {
			// 旧的无群组接口已移除，不能绕过群组约束。
			path := prefix + "/search/" + kind + "?keyword=meeting"
			request(path, owner.ID, http.StatusNotFound)
			for _, groupID := range []string{"0", "-1", "invalid", "18446744073709551616"} {
				request(prefix+"/groups/"+groupID+"/search/"+kind+"?keyword=meeting", owner.ID, http.StatusBadRequest)
			}
			path = fmt.Sprintf("%s/groups/%d/search/%s", prefix, first.ID, kind)
			for _, query := range []string{"", "?keyword=%20%20", "?keyword=meeting&page=-1", "?keyword=meeting&page_size=101", "?keyword=meeting&page_size=-1", "?keyword=meeting&page=invalid", "?keyword=meeting&page=9223372036854775807&page_size=100"} {
				request(path+query, owner.ID, http.StatusBadRequest)
			}
			request(path+fmt.Sprintf("?keyword=absent&user_id=%d", owner.ID), outsider.ID, http.StatusForbidden)
			request(fmt.Sprintf("%s/groups/999999/search/%s?keyword=meeting", prefix, kind), owner.ID, http.StatusForbidden)
			// page=0 使用默认第一页，与 Todo / Event 一致。
			page := request(path+"?keyword=absent&page=0", owner.ID, http.StatusOK)
			if page.Total != 0 || page.Items == nil || len(page.Items) != 0 || page.Page != 1 || page.PageSize != 20 {
				t.Fatalf("empty group or defaults: %+v", page)
			}
		}
	}
}

func TestSearchRetainsDepartingCreatorsRecords(t *testing.T) {
	for _, ownerQuits := range []bool{false, true} {
		name := "member quits"
		if ownerQuits {
			name = "owner transfers and quits"
		}
		t.Run(name, func(t *testing.T) {
			db, owner, member, _, first, _ := groupManagementFixture(t)
			departingID, remainingID := member.ID, owner.ID
			var target *uint
			if ownerQuits {
				departingID, remainingID = owner.ID, member.ID
				target = &remainingID
			}
			ctx := context.Background()
			start := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
			retainedTodo, err := todo.NewService(todo.NewGORMRepository(db)).Create(ctx, todo.CreateCommand{
				GroupID: first.ID, CreatorID: departingID, Title: "退群保留", StartsAt: &start, RepeatMode: model.RepeatOnce,
			})
			if err != nil {
				t.Fatal(err)
			}
			retainedEvent, err := event.NewService(event.NewGORMRepository(db)).Create(ctx, event.CreateCommand{
				GroupID: first.ID, CreatorID: departingID, Title: "退群保留", StartsAt: start, EndsAt: start.Add(time.Hour), RepeatMode: model.RepeatOnce,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := group.NewService(group.NewGORMRepository(db)).QuitGroup(ctx, first.ID, departingID, target); err != nil {
				t.Fatal(err)
			}
			// 实际退群只移除成员资格和授权，不改变记录的群组或创建者。
			var loadedTodo model.Todo
			var loadedEvent model.Event
			if err := db.First(&loadedTodo, retainedTodo.ID).Error; err != nil || loadedTodo.GroupID != first.ID || loadedTodo.CreatorID != departingID {
				t.Fatalf("quit changed or removed Todo: %+v err=%v", loadedTodo, err)
			}
			if err := db.First(&loadedEvent, retainedEvent.ID).Error; err != nil || loadedEvent.GroupID != first.ID || loadedEvent.CreatorID != departingID {
				t.Fatalf("quit changed or removed Event: %+v err=%v", loadedEvent, err)
			}
			request := groupSearchRequest(t, db)
			for _, kind := range []string{"todos", "events", "all"} {
				path := fmt.Sprintf("/api/groups/%d/search/%s?keyword=退群保留", first.ID, kind)
				page := request(path, remainingID, http.StatusOK)
				want := int64(1)
				if kind == "all" {
					want = 2
				}
				if page.Total != want || int64(len(page.Items)) != want {
					t.Fatalf("remaining member cannot find departing creator's records: %+v", page)
				}
				for _, item := range page.Items {
					if item.GroupID != first.ID || (item.Kind == "todo" && item.ID != retainedTodo.ID) || (item.Kind == "event" && item.ID != retainedEvent.ID) {
						t.Fatalf("unexpected retained result: %+v", item)
					}
				}
				request(path, departingID, http.StatusForbidden)
			}
		})
	}
}
