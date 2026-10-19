package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"note/internal/model"
	"note/internal/notification"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestNotificationSnapshotToCache(t *testing.T) {
	db, owner, actor, group := groupSchemaFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	// 超过旧的 50 条窗口，包含已读通知；全部缓存，未读数不等于列表长度。
	const total = 60
	notices := make([]model.Notification, 0, total+1)
	for i := 0; i < total; i++ {
		n := model.Notification{ReceiverID: owner.ID, ActorID: actor.ID, GroupID: group.ID, Type: model.Joined, CreatedAt: now}
		if i >= 3 {
			n.ReadAt = &now
		}
		notices = append(notices, n)
	}
	notices = append(notices, model.Notification{ReceiverID: actor.ID, ActorID: owner.ID, GroupID: group.ID, Type: model.Joined, CreatedAt: now})
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notices).Error; err != nil {
		t.Fatal(err)
	}
	// 超过浮点数精确范围的版本，必须保持十进制字符串精度。
	const revision = uint64(1<<53 + 1)
	states := []model.NotificationState{
		{UserID: owner.ID, Revision: revision, UnreadCount: 3},
		{UserID: actor.ID, Revision: 1, UnreadCount: 1},
	}
	if err := db.Omit("User").Create(&states).Error; err != nil {
		t.Fatal(err)
	}
	counts := 0
	if err := db.Callback().Query().After("gorm:query").Register("test:snapshot_count", func(q *gorm.DB) {
		sql := strings.ToLower(q.Statement.SQL.String())
		if strings.Contains(sql, "count(") && strings.Contains(sql, "notifications") {
			counts++
		}
	}); err != nil {
		t.Fatal(err)
	}
	repo := notification.NewGORMRepository(db)
	inbox, err := repo.LoadSnapshot(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if counts != 0 || inbox.Revision != revision || inbox.UnreadCount != 3 || len(inbox.Items) != total {
		t.Fatalf("wrong snapshot or extra COUNT: %+v, counts=%d", inbox, counts)
	}
	if inbox.Items[0].ID != notices[total-1].ID || inbox.Items[total-1].ID != notices[0].ID ||
		inbox.Items[0].ActorName != actor.Nickname || inbox.Items[0].GroupName != group.Name {
		t.Fatalf("wrong order, receiver isolation, or relations: %+v", inbox.Items)
	}
	for i, card := range inbox.Items {
		if card.ID != notices[total-1-i].ID {
			t.Fatalf("missing or incorrectly ordered notification at %d: %+v", i, card)
		}
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := notification.NewRedisCache(client)
	if installed, err := cache.InitSnapshot(ctx, owner.ID, inbox); err != nil || !installed {
		t.Fatalf("install: %v %v", installed, err)
	}
	prefix := fmt.Sprintf("note:notifications:v2:{%d}", owner.ID)
	state, err := client.HGetAll(ctx, prefix+":state").Result()
	if err != nil || state["revision"] != strconv.FormatUint(revision, 10) || state["unread_count"] != "3" {
		t.Fatalf("cached state: %v %v", state, err)
	}
	ids, err := client.ZRevRange(ctx, prefix+":order", 0, -1).Result()
	if err != nil || len(ids) != total {
		t.Fatalf("cached order: %v %v", ids, err)
	}
	for i, card := range inbox.Items {
		if ids[i] != strconv.FormatUint(uint64(card.ID), 10) {
			t.Fatalf("wrong cached sort: %v", ids)
		}
		data, err := client.HGet(ctx, prefix+":items", ids[i]).Bytes()
		if err != nil {
			t.Fatal(err)
		}
		var saved notification.Card
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved, card) {
			t.Fatalf("cached card changed: %+v want %+v", saved, card)
		}
	}
	before := server.Dump()
	if installed, err := cache.InitSnapshot(ctx, owner.ID, notification.Inbox{Revision: 0}); err != nil || installed {
		t.Fatalf("old snapshot overwrote cache: %v %v", installed, err)
	}
	if server.Dump() != before {
		t.Fatal("rejected initialization changed cache")
	}
	other, err := repo.LoadSnapshot(ctx, actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Items) != 1 || other.Items[0].ID != notices[total].ID {
		t.Fatalf("other user's snapshot is not isolated: %+v", other)
	}
	if installed, err := cache.InitSnapshot(ctx, actor.ID, other); err != nil || !installed {
		t.Fatalf("other user: %v %v", installed, err)
	}
	if got := client.HGet(ctx, prefix+":state", "revision").Val(); got != state["revision"] {
		t.Fatal("other user changed owner's state")
	}
	if n := client.Exists(ctx, prefix+":history").Val(); n != 0 {
		t.Fatal("snapshot fabricated event history")
	}
}

func TestNotificationSnapshotEmptyAndMissingState(t *testing.T) {
	db, owner, actor, _ := groupSchemaFixture(t)
	initializeJoinNotificationStates(t, db, owner)
	repo := notification.NewGORMRepository(db)
	ctx := context.Background()
	inbox, err := repo.LoadSnapshot(ctx, owner.ID)
	if err != nil || inbox.Items == nil || len(inbox.Items) != 0 || inbox.Revision != 0 || inbox.UnreadCount != 0 {
		t.Fatalf("empty snapshot: %+v %v", inbox, err)
	}
	for _, input := range []uint{0, actor.ID} {
		if _, err := repo.LoadSnapshot(ctx, input); err == nil {
			t.Fatalf("expected error for %+v", input)
		}
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := notification.NewRedisCache(client)
	if ok, err := cache.InitSnapshot(ctx, owner.ID, inbox); err != nil || !ok {
		t.Fatalf("empty install: %v %v", ok, err)
	}
	prefix := fmt.Sprintf("note:notifications:v2:{%d}", owner.ID)
	if got := client.HGet(ctx, prefix+":state", "unread_count").Val(); got != "0" {
		t.Fatalf("empty cache missing state: %q", got)
	}
	if ok, err := cache.InitSnapshot(ctx, owner.ID, inbox); err != nil || ok {
		t.Fatalf("empty cache not recognized: %v %v", ok, err)
	}
}
