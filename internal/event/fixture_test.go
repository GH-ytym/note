package event

import (
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"note/internal/model"
	"path/filepath"
	"testing"
)

func eventRepositoryFixture(t *testing.T) (*gorm.DB, model.User, model.Group) {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "events.db"))
	db, err := gorm.Open(gormlite.Open("file:"+path+"?_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.Group{}, &model.GroupMember{}, &model.Event{}, &model.EventDate{}, &model.EventMember{}); err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "owner", Suffix: 12345, Email: "owner@example.com", PasswordHash: "test-hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	group := model.Group{Name: "日程测试群", OwnerID: owner.ID, Code: "ABC123"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GroupMember{GroupID: group.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatal(err)
	}
	return db, owner, group
}
