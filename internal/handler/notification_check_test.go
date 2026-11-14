package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gin-gonic/gin"
	"note/internal/middleware"
	"note/internal/notification"
)

type checkService struct {
	notification.Service
	load     func(context.Context) (notification.SnapshotPage, error)
	revision func(context.Context) (uint64, error)
}

func (s checkService) SnapshotPage(ctx context.Context, uid, before, limit uint) (notification.SnapshotPage, error) {
	return s.load(ctx)
}
func (s checkService) Revision(ctx context.Context, uid uint) (uint64, error) { return s.revision(ctx) }

func TestStreamPeriodicCheck(t *testing.T) {
	for _, mode := range []string{"behind", "equal", "retry", "disconnect", "expired", "timeout", "snapshot failure"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				hub := notification.NewHub()
				defer hub.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				reads, checks := 0, 0
				canceled := false
				svc := checkService{
					load: func(ctx context.Context) (notification.SnapshotPage, error) {
						reads++
						if reads > 1 && mode == "snapshot failure" {
							return notification.SnapshotPage{}, errors.New("query failed")
						}
						rev := uint64(12)
						if reads > 1 {
							rev = 13
						}
						return notification.SnapshotPage{Revision: rev, UnreadCount: 7, Items: []notification.Card{}}, nil
					},
					revision: func(ctx context.Context) (uint64, error) {
						checks++
						if mode == "disconnect" || mode == "expired" || mode == "timeout" {
							<-ctx.Done()
							canceled = true
							return 0, ctx.Err()
						}
						if mode == "retry" && checks == 1 {
							return 0, errors.New("temporary failure")
						}
						if mode == "equal" {
							return 12, nil
						}
						return 13, nil
					},
				}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(streamWriter{rec})
				c.Request = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
				c.Set(middleware.UserIDKey, uint(8))
				exp := time.Now().Add(10 * time.Minute)
				if mode == "expired" {
					exp = time.Now().Add(62 * time.Second)
				}
				c.Set(middleware.AccessExpiresAtKey, exp)
				done := make(chan struct{})
				go func() { defer close(done); NewNotificationHandler(svc, hub).Stream(c) }()
				synctest.Wait()
				if reads != 1 || checks != 0 {
					t.Fatalf("initial reads=%d checks=%d", reads, checks)
				}
				time.Sleep(60 * time.Second)
				synctest.Wait()
				if checks != 1 {
					t.Fatalf("expected first scheduled check, got %d", checks)
				}
				switch mode {
				case "behind":
					if reads != 2 || strings.Count(rec.Body.String(), "notification.snapshot") != 2 {
						t.Fatal("missing periodic snapshot")
					}
					if !strings.Contains(rec.Body.String(), `"revision":"13"`) {
						t.Fatal("wrong corrected version")
					}
					time.Sleep(60 * time.Second)
					synctest.Wait()
					if reads != 2 {
						t.Fatal("resent snapshot after catching up")
					}
				case "equal":
					if reads != 1 {
						t.Fatal("queried cards while current")
					}
				case "retry":
					if reads != 1 {
						t.Fatal("unexpected refresh after failed check")
					}
					time.Sleep(60 * time.Second)
					synctest.Wait()
					if checks != 2 || reads != 2 {
						t.Fatal("did not retry revision query")
					}
				case "disconnect":
					cancel()
					synctest.Wait()
					if !canceled {
						t.Fatal("query ignored disconnect")
					}
				case "expired":
					time.Sleep(2 * time.Second)
					synctest.Wait()
					if !canceled {
						t.Fatal("query ignored token expiry")
					}
				case "timeout":
					time.Sleep(5 * time.Second)
					synctest.Wait()
					if !canceled {
						t.Fatal("query ignored five-second timeout")
					}
					select {
					case <-done:
						t.Fatal("check timeout closed stream")
					default:
					}
				case "snapshot failure":
					select {
					case <-done:
					default:
						t.Fatal("refresh failure did not terminate stream")
					}
					if strings.Count(rec.Body.String(), "notification.snapshot") != 1 {
						t.Fatal("sent failed snapshot")
					}
				}
				cancel()
				synctest.Wait()
				select {
				case <-done:
				default:
					t.Fatal("stream did not stop")
				}
			})
		})
	}
}
