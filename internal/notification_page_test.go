package internal

import (
	"context"
	"note/internal/model"
	"note/internal/notification"
	"testing"
)

func TestNotificationDatabasePage(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	for i := 0; i < 5; i++ {
		n := model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined}
		if err := db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	repo := notification.NewGORMRepository(db)
	seen := map[uint]bool{}
	var before uint
	for {
		p, err := repo.LoadPage(context.Background(), owner.ID, before, 2)
		if err != nil {
			t.Fatal(err)
		}
		if p.UnreadCount != 5 || len(p.Items) > 2 {
			t.Fatalf("page: %+v", p)
		}
		for _, card := range p.Items {
			if seen[card.ID] || (before != 0 && card.ID >= before) {
				t.Fatalf("duplicate or wrong cursor: %d", card.ID)
			}
			seen[card.ID] = true
		}
		if p.Next == 0 {
			break
		}
		before = p.Next
	}
	if len(seen) != 5 {
		t.Fatalf("missing notices: %v", seen)
	}
	p, err := repo.LoadPage(context.Background(), member.ID, 0, 2)
	if err != nil || len(p.Items) != 0 || p.UnreadCount != 0 {
		t.Fatalf("isolation: %+v %v", p, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.LoadPage(ctx, owner.ID, 0, 2); err == nil {
		t.Fatal("ignored cancellation")
	}
	var rows []struct{ Detail string }
	if err := db.Raw("EXPLAIN QUERY PLAN SELECT * FROM notifications WHERE receiver_id = ? AND id < ? ORDER BY id DESC LIMIT 3", owner.ID, 999).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Log(row.Detail)
	}
	if !db.Migrator().HasIndex(&model.Notification{}, "idx_notifications_receiver_id") {
		t.Fatal("missing page index")
	}
}
