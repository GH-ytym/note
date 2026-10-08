package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/model"
	"note/internal/notification"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
)

const noteAppTestSecret = "test-only-secret-0123456789abcdef"

// 在独立测试进程中运行真正的 app，避免信号和 stdin 影响其他测试。
func TestNotificationAppProcess(t *testing.T) {
	if os.Getenv("NOTE_APP_TEST_PROCESS") != "1" {
		return
	}
	if err := Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type notificationTestApp struct {
	stdin  io.WriteCloser
	ready  chan string
	done   chan struct{}
	stderr bytes.Buffer
	err    error
}

func startNotificationTestApp(t *testing.T, dbPath, redisAddr string) *notificationTestApp {
	t.Helper()
	app := &notificationTestApp{ready: make(chan string, 1), done: make(chan struct{})}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNotificationAppProcess$")
	// 只使用临时数据库、测试 Redis 和测试密钥；移除宿主的业务配置。
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NOTE_") &&
			!strings.HasPrefix(entry, "HTTP_ADDR=") &&
			!strings.HasPrefix(entry, "GIN_MODE=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env,
		"NOTE_APP_TEST_PROCESS=1",
		"NOTE_DB_PATH="+dbPath,
		"NOTE_REDIS_ADDR="+redisAddr,
		"NOTE_JWT_SECRET="+noteAppTestSecret,
		"NOTE_STOP_ON_STDIN_CLOSE=1",
		"HTTP_ADDR=127.0.0.1:0",
		"GIN_MODE=release",
	)
	cmd.Stderr = &app.stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	app.stdin, err = cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if url, ok := strings.CutPrefix(scanner.Text(), "NOTE_SERVER_URL="); ok {
				app.ready <- url
			}
		}
		app.err = errors.Join(cmd.Wait(), scanner.Err())
		close(app.done)
	}()
	t.Cleanup(func() {
		_ = app.stdin.Close()
		select {
		case <-app.done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-app.done
			t.Error("test app did not stop after stdin closed")
		}
	})
	return app
}

func (app *notificationTestApp) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-app.done:
		return app.err
	case <-time.After(15 * time.Second):
		t.Fatal("test app did not finish")
		return nil
	}
}

// Redis 能连接但不允许订阅时，不能向桌面端报告启动成功。
func TestAppDoesNotReportReadyWhenSubscriptionFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisServer.Server().SetPreHook(func(peer *server.Peer, command string, _ ...string) bool {
		if strings.EqualFold(command, "PSUBSCRIBE") {
			peer.WriteError("ERR test subscription rejected")
			return true
		}
		return false
	})
	app := startNotificationTestApp(t, filepath.Join(t.TempDir(), "note.db"), redisServer.Addr())
	if err := app.wait(t); err == nil {
		t.Fatal("app started despite failed subscription")
	}
	if !strings.Contains(app.stderr.String(), "initialize notification subscriber") {
		t.Fatalf("unexpected startup error: %s", app.stderr.String())
	}
	select {
	case url := <-app.ready:
		t.Fatalf("app reported ready before subscribing: %s", url)
	default:
	}
}

func openNotificationTestStream(
	t *testing.T, ctx context.Context, url, token string,
) <-chan notification.Event {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/notifications/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("open notification stream: %s", res.Status)
	}
	scanner := bufio.NewScanner(res.Body)
	if !scanner.Scan() || scanner.Text() != ": connected" {
		t.Fatalf("missing SSE connection confirmation: %v", scanner.Err())
	}
	events := make(chan notification.Event, 2)
	go func() {
		defer close(events)
		var event notification.Event
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "id:"):
				event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "event:"):
				event.Name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				event.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			case line == "" && event.ID != "":
				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
				event = notification.Event{}
			}
		}
	}()
	return events
}

// 走真实 app 启动、HTTP 入群、Outbox、Redis、Hub、SSE，并保持 SSE 在线关闭 app。
func TestAppPushesJoinApplicationAndShutsDown(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	if err := db.Model(&item).Updates(map[string]any{"policy": model.Approval, "code": "ABC123"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatal(err)
	}
	var databases []struct{ Name, File string }
	if err := db.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
		t.Fatal(err)
	}
	var dbPath string
	for _, database := range databases {
		if database.Name == "main" {
			dbPath = database.File
		}
	}
	if dbPath == "" {
		t.Fatal("test database path is empty")
	}
	redisServer := miniredis.RunT(t)
	app := startNotificationTestApp(t, dbPath, redisServer.Addr())
	var url string
	select {
	case url = <-app.ready:
	case <-app.done:
		t.Fatalf("app failed to start: %v: %s", app.err, app.stderr.String())
	case <-time.After(15 * time.Second):
		t.Fatal("app did not report ready")
	}
	if redisServer.PubSubNumPat() != 1 {
		t.Fatal("app reported ready without a Redis subscription")
	}
	tokens, err := auth.NewTokenManager(noteAppTestSecret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ownerToken, err := tokens.Generate(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, err := tokens.Generate(member.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	firstWindow := openNotificationTestStream(t, ctx, url, ownerToken)
	secondWindow := openNotificationTestStream(t, ctx, url, ownerToken)
	otherUser := openNotificationTestStream(t, ctx, url, memberToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		url+"/api/groups/join", strings.NewReader(`{"code":"ABC123"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("join application: %s", res.Status)
	}
	var pending model.GroupJoinRequest
	if err := json.NewDecoder(res.Body).Decode(&pending); err != nil {
		t.Fatal(err)
	}
	var notice model.Notification
	if err := db.Where("join_request_id = ? AND receiver_id = ?", pending.ID, owner.ID).
		First(&notice).Error; err != nil {
		t.Fatal(err)
	}
	var firstEvent notification.Event
	for _, window := range []<-chan notification.Event{firstWindow, secondWindow} {
		select {
		case event, ok := <-window:
			if !ok || event.ID == "" || event.Name != "notification.created" {
				t.Fatalf("missing notification event: %+v", event)
			}
			var card notification.Card
			if err := json.Unmarshal([]byte(event.Data), &card); err != nil {
				t.Fatal(err)
			}
			if card.ID != notice.ID || card.Type != model.JoinRequested ||
				card.GroupID != item.ID || card.ActorID != member.ID || card.RequestStatus != model.Pending {
				t.Fatalf("wrong notification card: %+v", card)
			}
			assertNotificationCardFields(t, []byte(event.Data), model.Pending)
			if firstEvent.ID != "" && firstEvent != event {
				t.Fatal("two windows received different events")
			}
			firstEvent = event
		case <-ctx.Done():
			t.Fatal("notification did not reach SSE")
		}
	}
	var task model.Outbox
	if err := db.First(&task).Error; err != nil {
		t.Fatal(err)
	}
	waitOutboxPublished(t, db, task.ID)
	// SSE 仍在线时关闭父进程的 stdin，验证正常清理而非等待连接超时。
	_ = app.stdin.Close()
	if err := app.wait(t); err != nil {
		t.Fatalf("app shutdown: %v: %s", err, app.stderr.String())
	}
	select {
	case event, ok := <-otherUser:
		if ok {
			t.Fatalf("notification leaked to another user: %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("SSE stayed open after app shutdown")
	}
	if redisServer.PubSubNumPat() != 0 {
		t.Fatal("app shutdown left a Redis subscription")
	}
}
