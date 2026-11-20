package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/middleware"
	"note/internal/notification"

	"github.com/gin-gonic/gin"
)

func TestStreamCalendarKeepsNotificationRevision(t *testing.T) {
	hub := notification.NewHub()
	defer hub.Close()
	calls := 0
	data := `{"group_id":3,"kind":"todo","id":42,"action":"upsert"}`
	svc := streamService{load: func(context.Context) (notification.SnapshotPage, error) {
		calls++
		// Both events arrive while the initial snapshot is being loaded.
		hub.Send(8, notification.Event{Name: "cldr", Data: data})
		hub.Send(8, notification.Event{Name: "notification.change", Data: `{"revision":"13","upserts":[],"removed_ids":[],"unread_count":1}`})
		hub.Close()
		return notification.SnapshotPage{Revision: 12, Items: []notification.Card{}}, nil
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(streamWriter{rec})
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(middleware.UserIDKey, uint(8))
	c.Set(middleware.AccessExpiresAtKey, time.Now().Add(time.Minute))
	NewNotificationHandler(svc, hub).Stream(c)
	body := rec.Body.String()
	if calls != 1 || strings.Count(body, "event:notification.snapshot") != 1 ||
		!strings.Contains(body, "event:cldr\ndata:"+data) ||
		!strings.Contains(body, "event:notification.change\ndata:") {
		t.Fatalf("snapshot calls=%d, stream=%s", calls, body)
	}
}
