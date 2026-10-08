package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	apperrors "note/internal/errors"
	"note/internal/group"
	"note/internal/model"
)

func TestFixedGroupCodeLookupAndJoin(t *testing.T) {
	for _, policy := range []model.GroupPolicy{model.Public, model.Approval, model.Restricted, model.Personal} {
		t.Run(string(policy), func(t *testing.T) {
			db, owner, member, item := groupSchemaFixture(t)
			if err := db.Model(&item).Updates(map[string]any{"code": "ABC123", "policy": policy}).Error; err != nil {
				t.Fatal(err)
			}
			request := notificationRequest(t, db)
			res := request(http.MethodGet, "/api/groups/lookup?code=abc123", member.ID, "")
			want := http.StatusOK
			if policy == model.Personal {
				want = http.StatusNotFound
			}
			if res.Code != want {
				t.Fatalf("lookup %d %s", res.Code, res.Body.String())
			}
			if want == http.StatusOK {
				var fields map[string]any
				if err := json.Unmarshal(res.Body.Bytes(), &fields); err != nil {
					t.Fatal(err)
				}
				if len(fields) != 3 || fields["name"] != item.Name || fields["code"] != "ABC123" || fields["policy"] != string(policy) {
					t.Fatalf("preview leaked fields: %v", fields)
				}
			}
			res = request(http.MethodPost, "/api/groups/join", member.ID, `{"code":" abc123 "}`)
			switch policy {
			case model.Public:
				want = 200
			case model.Approval:
				want = 202
			case model.Restricted:
				want = 403
			case model.Personal:
				want = 404
			}
			if res.Code != want {
				t.Fatalf("join %d %s", res.Code, res.Body.String())
			}
			var members, notices int64
			db.Model(&model.GroupMember{}).Where("group_id=? AND user_id=?", item.ID, member.ID).Count(&members)
			db.Model(&model.Notification{}).Where("receiver_id=?", owner.ID).Count(&notices)
			if (members == 1) != (policy == model.Public) {
				t.Fatalf("membership=%d", members)
			}
			if (notices == 1) != (policy == model.Public || policy == model.Approval) {
				t.Fatalf("notices=%d", notices)
			}
			for _, code := range []string{"", "TOOLONG", "ABC!23"} {
				if got := request(http.MethodPost, "/api/groups/join", member.ID, fmt.Sprintf(`{"code":%q}`, code)); got.Code != 400 {
					t.Fatalf("invalid code: %d", got.Code)
				}
			}
			for _, path := range []string{fmt.Sprintf("/api/groups/%d/refresh", item.ID), fmt.Sprintf("/api/groups/%d/join", item.ID)} {
				if got := request(http.MethodPost, path, owner.ID, `{"code":"ABC123"}`); got.Code != 404 {
					t.Fatalf("old route remains: %s %d", path, got.Code)
				}
			}
		})
	}
}

func TestGroupCodeMigrationRepairsDuplicatesWithoutChangingValidCodes(t *testing.T) {
	db, owner, _, first := groupSchemaFixture(t)
	if err := db.Exec("DROP INDEX idx_groups_code").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&first).Update("code", "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"ABC123", "", "bad", "ZZZ999"} {
		if err := db.Create(&model.Group{Name: "legacy", OwnerID: owner.ID, Code: code}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var before, after []model.Group
	db.Order("id").Find(&before)
	seen := map[string]bool{}
	for _, item := range before {
		if len(item.Code) != 6 || seen[item.Code] {
			t.Fatalf("invalid code: %v", item)
		}
		seen[item.Code] = true
	}
	if before[0].Code != "ABC123" || before[4].Code != "ZZZ999" {
		t.Fatal("valid codes changed")
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	db.Order("id").Find(&after)
	for i := range before {
		if before[i].Code != after[i].Code {
			t.Fatal("migration not idempotent")
		}
	}
	duplicate := model.Group{Name: "collision", OwnerID: owner.ID, Code: "ABC123", Members: []model.GroupMember{{UserID: owner.ID}}}
	err := group.NewGORMRepository(db).Create(context.Background(), &duplicate)
	if !errors.Is(err, apperrors.ErrGroupCodeConflict) {
		t.Fatalf("collision classification: %v", err)
	}
}
