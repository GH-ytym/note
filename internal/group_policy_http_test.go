package internal

import (
	"encoding/json"
	"net/http"
	"note/internal/model"
	"testing"
)

func TestCreateGroupPolicies(t *testing.T) {
	db, owner, _, _ := groupSchemaFixture(t)
	request := notificationRequest(t, db)
	for _, policy := range []string{"public", "approval", "restricted", "personal", "invalid"} {
		res := request(http.MethodPost, "/api/groups", owner.ID, `{"name":"权限测试","policy":"`+policy+`"}`)
		if policy == "personal" || policy == "invalid" {
			if res.Code != 400 {
				t.Fatalf("%s: %d %s", policy, res.Code, res.Body.String())
			}
			continue
		}
		if res.Code != 201 {
			t.Fatalf("%s: %d %s", policy, res.Code, res.Body.String())
		}
		var item model.Group
		if err := json.Unmarshal(res.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		var saved model.Group
		if err := db.First(&saved, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		if string(saved.Policy) != policy {
			t.Fatalf("saved policy %s", saved.Policy)
		}
		var count int64
		db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", item.ID, owner.ID).Count(&count)
		if count != 1 {
			t.Fatal("missing owner membership")
		}
	}
}
