package handler

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"note/internal/middleware"
	"note/internal/notification"
	"strings"
	"testing"
	"time"
)

type streamService struct {
	notification.Service
	load func(context.Context) (notification.SnapshotPage, error)
}

func (s streamService) SnapshotPage(ctx context.Context, uid, before, limit uint) (notification.SnapshotPage, error) {
	return s.load(ctx)
}

type streamWriter struct{ *httptest.ResponseRecorder }

func (w streamWriter) SetWriteDeadline(time.Time) error { return nil }

func TestStreamQueuedEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []uint64
		second uint64
		reset  bool
		want   string
	}{
		{"duplicate", []uint64{12, 13, 13, 14}, 0, false, "12,13,14"},
		{"gap", []uint64{15, 16}, 15, false, "12,15,16"},
		{"behind", []uint64{15}, 14, false, "12"},
		{"reset", []uint64{14}, 13, true, "12,13,14"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := notification.NewHub()
			defer hub.Close()
			calls := 0
			svc := streamService{load: func(ctx context.Context) (notification.SnapshotPage, error) {
				calls++
				if calls == 1 {
					if tc.reset {
						hub.Send(8, notification.Event{Name: "notification.reset", Data: "{}"})
					}
					for _, rev := range tc.events {
						data, _ := json.Marshal(notification.Change{Revision: rev})
						hub.Send(8, notification.Event{Name: "notification.change", Data: string(data)})
					}
					hub.Close()
					return notification.SnapshotPage{Revision: 12, Items: []notification.Card{}, UnreadCount: 38}, nil
				}
				return notification.SnapshotPage{Revision: tc.second, Items: []notification.Card{}}, nil
			}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(streamWriter{rec})
			c.Request = httptest.NewRequest("GET", "/", nil)
			c.Set(middleware.UserIDKey, uint(8))
			c.Set(middleware.AccessExpiresAtKey, time.Now().Add(time.Minute))
			NewNotificationHandler(svc, hub).Stream(c)
			var got []string
			for _, line := range strings.Split(rec.Body.String(), "\n") {
				if strings.HasPrefix(line, "data:") {
					var body map[string]json.RawMessage
					if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &body); err != nil {
						t.Fatal(err)
					}
					var rev string
					if err := json.Unmarshal(body["revision"], &rev); err != nil {
						t.Fatal(err)
					}
					got = append(got, rev)
				}
			}
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("versions %v want %s; body %s", got, tc.want, rec.Body.String())
			}
		})
	}
}

func TestStreamOverflowAndCancellation(t *testing.T) {
	for _, overflow := range []bool{false, true} {
		hub := notification.NewHub()
		defer hub.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		svc := streamService{load: func(readCtx context.Context) (notification.SnapshotPage, error) {
			if !overflow {
				cancel()
				<-readCtx.Done()
				return notification.SnapshotPage{}, readCtx.Err()
			}
			for rev := uint64(1); rev <= 33; rev++ {
				data, _ := json.Marshal(notification.Change{Revision: rev})
				hub.Send(8, notification.Event{Name: "notification.change", Data: string(data)})
			}
			return notification.SnapshotPage{Revision: 33, Items: []notification.Card{}}, nil
		}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(streamWriter{rec})
		c.Request = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		c.Set(middleware.UserIDKey, uint(8))
		c.Set(middleware.AccessExpiresAtKey, time.Now().Add(time.Minute))
		done := make(chan struct{})
		go func() { defer close(done); NewNotificationHandler(svc, hub).Stream(c) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("stream did not terminate")
		}
		if !overflow && rec.Body.Len() != 0 {
			t.Fatal("sent canceled snapshot")
		}
		if overflow && strings.Count(rec.Body.String(), "notification.snapshot") != 1 {
			t.Fatal(rec.Body.String())
		}
	}
}
