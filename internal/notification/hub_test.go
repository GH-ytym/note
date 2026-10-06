package notification

import (
	"strconv"
	"testing"
)

func TestHubSlowConnectionCanSubscribeAgain(t *testing.T) {
	hub := NewHub()
	t.Cleanup(hub.Close)
	slow, cancelSlow, err := hub.Subscribe(8)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancelSlow)
	fast, cancelFast, err := hub.Subscribe(8)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancelFast)

	// 慢窗口不接收；快窗口每次都读取，只有慢窗口的队列会满。
	for i := 1; i <= cap(slow)+1; i++ {
		event := Event{ID: strconv.Itoa(i), Name: "notification.created", Data: `{}`}
		hub.Send(8, event)
		select {
		case got, ok := <-fast:
			if !ok || got != event {
				t.Fatalf("fast connection lost event %d: %+v ok=%t", i, got, ok)
			}
		default:
			t.Fatalf("fast connection did not receive event %d", i)
		}
	}
	// 被关闭的 channel 会先读完已有缓冲，但缺少导致溢出的那条事件。
	for i := 1; i <= cap(slow); i++ {
		select {
		case got, ok := <-slow:
			if !ok || got.ID != strconv.Itoa(i) {
				t.Fatalf("unexpected buffered event %d: %+v ok=%t", i, got, ok)
			}
		default:
			t.Fatalf("buffered event %d missing", i)
		}
	}
	select {
	case _, ok := <-slow:
		if ok {
			t.Fatal("overflowed subscription stayed open")
		}
	default:
		t.Fatal("overflowed subscription was not closed")
	}
	cancelSlow() // 已由 Send 清理，重复取消也不能 panic。
	cancelSlow()

	reconnected, cancelReconnected, err := hub.Subscribe(8)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancelReconnected)
	if reconnected == slow {
		t.Fatal("reused a closed channel")
	}
	select {
	case <-reconnected:
		t.Fatal("Hub unexpectedly replayed history")
	default:
	}
	// 新订阅恢复接收后续事件；缺少的历史需要另外补收。
	event := Event{ID: "34", Name: "notification.created", Data: `{}`}
	hub.Send(8, event)
	for _, connection := range []<-chan Event{fast, reconnected} {
		select {
		case got, ok := <-connection:
			if !ok || got != event {
				t.Fatalf("active connection missed event: %+v ok=%t", got, ok)
			}
		default:
			t.Fatal("active connection did not receive the next event")
		}
	}
}
