package internal

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"note/internal/group"
	"note/internal/model"
)

func TestGroupReviewHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"accept", `{"decision":"accept"}`, 204},
		{"reject", `{"decision":"reject"}`, 204},
		{"anonymous", `{"decision":"accept"}`, 401},
		{"not_owner", `{"decision":"accept","user_id":1}`, 403},
		{"missing", `{"decision":"accept"}`, 404},
		{"invalid_id", `{"decision":"accept"}`, 400},
		{"zero_id", `{"decision":"accept"}`, 400},
		{"invalid_decision", `{"decision":"accepted"}`, 400},
		{"empty", `{}`, 400},
		{"malformed", `{`, 400},
		{"expired", `{"decision":"accept"}`, 409},
		{"repeat", `{"decision":"reject"}`, 409},
		{"legacy_prefix", `{"decision":"accept"}`, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, owner, user, g := groupSchemaFixture(t)
			initializeJoinNotificationStates(t, db, owner, user)
			if err := db.Model(&g).Updates(map[string]any{"code": "ABC123", "policy": model.Approval}).Error; err != nil {
				t.Fatal(err)
			}
			repo := group.NewGORMRepository(db)
			req, msg, err := repo.Join(context.Background(), user.ID, "ABC123")
			if err != nil {
				t.Fatal(err)
			}
			uid := owner.ID
			path := fmt.Sprintf("/api/groups/join-requests/%d/review", req.ID)
			switch tc.name {
			case "anonymous":
				uid = 0
			case "not_owner":
				uid = user.ID
			case "missing":
				path = "/api/groups/join-requests/99999/review"
			case "invalid_id":
				path = "/api/groups/join-requests/no/review"
			case "zero_id":
				path = "/api/groups/join-requests/0/review"
			case "legacy_prefix":
				path = fmt.Sprintf("/groups/join-requests/%d/review", req.ID)
			case "expired":
				if err := db.Model(req).Update("created_at", time.Now().Add(-8*24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			case "repeat":
				if err := repo.Review(context.Background(), owner.ID, req.ID, model.Accepted); err != nil {
					t.Fatal(err)
				}
			}
			res := notificationRequest(t, db)(http.MethodPost, path, uid, tc.body)
			if res.Code != tc.want {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			if tc.want == 204 && res.Body.Len() != 0 {
				t.Fatal("204 must have no body")
			}
			if err := db.First(req, req.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.want == 204 || tc.name == "repeat" {
				want := model.Accepted
				if tc.name == "reject" {
					want = model.Rejected
				}
				if req.Status != want {
					t.Fatalf("status=%s", req.Status)
				}
				assertJoinNotificationState(t, db, owner.ID, 2, 0)
				assertJoinNotificationState(t, db, user.ID, 1, 1)
				if err := db.First(msg, msg.ID).Error; err != nil {
					t.Fatal(err)
				}
				if msg.ReadAt == nil {
					t.Fatal("owner notice not read")
				}
			} else {
				if req.Status != model.Pending {
					t.Fatal("failed request changed application")
				}
				assertJoinNotificationState(t, db, owner.ID, 1, 1)
				assertJoinNotificationState(t, db, user.ID, 0, 0)
			}
		})
	}
}
