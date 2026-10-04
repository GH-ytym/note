package internal

import (
	"context"
	"testing"
	"time"

	"note/internal/group"
	"note/internal/model"
	"note/internal/todo"

	"gorm.io/gorm"
)

func TestReadOnlyQueriesDoNotWaitForWriter(t *testing.T) {
	db, service, item, owner, viewer, date := completionFixture(t)
	ctx := context.Background()
	if err := service.SetOccurrenceDone(ctx, item.ID, owner.ID, date, true); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Group{}).Where("id = ?", item.GroupID).Update("code", "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// 只在这个测试中使用多个连接，验证 WAL 的读写并发。
	sqlDB.SetMaxOpenConns(4)
	writer, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, "UPDATE todos SET title = ? WHERE id = ?", "尚未提交的标题", item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.ExecContext(ctx, "UPDATE groups SET code = ? WHERE id = ?", "ZZZZZZ", item.GroupID); err != nil {
		t.Fatal(err)
	}
	groups := group.NewService(group.NewGORMRepository(db))

	// 写事务一直未提交。所有读接口应立即读到已提交的数据，而不是抢写锁。
	for _, tc := range []struct {
		name string
		read func(*testing.T, context.Context)
	}{
		{"todo list", func(t *testing.T, ctx context.Context) {
			page, err := service.List(ctx, todo.ListQuery{GroupID: item.GroupID, UserID: owner.ID, Page: 1, PageSize: 10})
			if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Title != item.Title {
				t.Fatalf("list: %+v %v", page, err)
			}
		}},
		{"todo detail", func(t *testing.T, ctx context.Context) {
			loaded, err := service.Get(ctx, item.ID, owner.ID)
			if err != nil || loaded.Title != item.Title || loaded.Creator == nil || loaded.Creator.ID != owner.ID {
				t.Fatalf("detail: %+v %v", loaded, err)
			}
		}},
		{"completion list", func(t *testing.T, ctx context.Context) {
			users, err := service.GetOccurrenceCompletions(ctx, item.ID, owner.ID, date)
			if err != nil || len(users) != 1 || users[0].User.ID != owner.ID {
				t.Fatalf("completions: %+v %v", users, err)
			}
		}},
		{"group members", func(t *testing.T, ctx context.Context) {
			members, err := groups.ListMembers(ctx, item.GroupID, owner.ID)
			if err != nil || len(members) != 2 {
				t.Fatalf("members: %+v %v", members, err)
			}
		}},
		{"invite code", func(t *testing.T, ctx context.Context) {
			code, err := groups.GetInviteCode(ctx, item.GroupID, owner.ID)
			if err != nil || code != "ABC123" {
				t.Fatalf("invite: %q %v", code, err)
			}
		}},
		{"calendar", func(t *testing.T, ctx context.Context) {
			rows, err := service.CalendarOccurrences(ctx, owner.ID, date, date.AddDate(0, 0, 1))
			if err != nil || len(rows) != 1 || rows[0].Title != item.Title || !rows[0].OccurrenceDone {
				t.Fatalf("calendar: %+v %v", rows, err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 未指定只读时，会因获取写锁失败而超过这个期限。
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			tc.read(t, ctx)
		})
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	// 只读事务结束后，连接仍应可以用于写入。
	if err := service.SetOccurrenceDone(ctx, item.ID, viewer.ID, date, true); err != nil {
		t.Fatal(err)
	}
}

func TestCalendarReadsOneSnapshotDuringEdit(t *testing.T) {
	db, service, item, owner, _, date := completionFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	custom := model.RepeatCustom
	dates := []time.Time{date}
	item, err := service.Patch(ctx, item.ID, owner.ID, todo.PatchCommand{RepeatMode: &custom, CustomDates: &dates, Version: item.Version})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetOccurrenceDone(ctx, item.ID, owner.ID, date, true); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	paused, resume := make(chan struct{}), make(chan struct{})
	type calendarReadKey struct{}
	// 在读完 Todo、尚未 Preload 日期时暂停；让并发编辑确定发生在两次查询之间。
	if err := db.Callback().Query().After("gorm:query").Before("gorm:preload").Register("pause_calendar_snapshot", func(tx *gorm.DB) {
		if tx.Statement.Table == "todos" && tx.Statement.Context.Value(calendarReadKey{}) == true {
			close(paused)
			select {
			case <-resume:
			case <-ctx.Done():
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	type result struct {
		rows []todo.CalendarOccurrence
		err  error
	}
	results := make(chan result, 1)
	go func() {
		rows, err := service.CalendarOccurrences(context.WithValue(ctx, calendarReadKey{}, true), owner.ID, date, date.AddDate(0, 0, 2))
		results <- result{rows, err}
	}()
	select {
	case <-paused:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	newTitle := "修改后的标题"
	newDate := date.AddDate(0, 0, 1)
	newDates := []time.Time{newDate}
	writeErr := func() error {
		if _, err := service.Patch(ctx, item.ID, owner.ID, todo.PatchCommand{Title: &newTitle, CustomDates: &newDates, Version: item.Version}); err != nil {
			return err
		}
		if err := service.SetOccurrenceDone(ctx, item.ID, owner.ID, date, false); err != nil {
			return err
		}
		return service.SetOccurrenceDone(ctx, item.ID, owner.ID, newDate, true)
	}()
	close(resume)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	old := <-results
	if old.err != nil || len(old.rows) != 1 {
		t.Fatalf("old snapshot: %+v %v", old.rows, old.err)
	}
	row := old.rows[0]
	if row.Title != item.Title || row.Version != item.Version || row.OccursAt.Format(time.DateOnly) != date.Format(time.DateOnly) || !row.OccurrenceDone {
		t.Fatalf("mixed calendar snapshot: %+v", row)
	}
	// 事务结束后的新查询，应取得完整的新版本和新完成状态。
	fresh, err := service.CalendarOccurrences(ctx, owner.ID, date, date.AddDate(0, 0, 2))
	if err != nil || len(fresh) != 1 {
		t.Fatalf("new snapshot: %+v %v", fresh, err)
	}
	row = fresh[0]
	if row.Title != newTitle || row.Version != item.Version+1 || row.OccursAt.Format(time.DateOnly) != newDate.Format(time.DateOnly) || !row.OccurrenceDone {
		t.Fatalf("incorrect new calendar snapshot: %+v", row)
	}
}
