package internal

import (
	"testing"

	"note/internal/model"
)

func TestUserAvatarMigration(t *testing.T) {
	db := authTestDB(t)
	// 模拟加头像字段之前，已有账号的 users 表。
	if err := db.Exec(`CREATE TABLE users (
		id integer PRIMARY KEY AUTOINCREMENT,
		username text NOT NULL,
		suffix integer NOT NULL,
		email text NOT NULL,
		password_hash text NOT NULL,
		nickname text NOT NULL,
		created_at datetime,
		updated_at datetime,
		CONSTRAINT users_suffix_range CHECK (suffix >= 10000 AND suffix <= 99999)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, suffix, email, password_hash, nickname)
		VALUES (1, 'existing', 12345, 'existing@example.com', 'existing-hash', '旧用户')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	var existing model.User
	if err := db.First(&existing, 1).Error; err != nil {
		t.Fatal(err)
	}
	if existing.Avatar != "" || existing.Username != "existing" || existing.Suffix != 12345 ||
		existing.Email != "existing@example.com" || existing.PasswordHash != "existing-hash" || existing.Nickname != "旧用户" {
		t.Fatalf("avatar migration changed existing account: %#v", existing)
	}
	// 不指定 avatar 的新账号也应由数据库默认填充空字符串。
	if err := db.Exec(`INSERT INTO users (username, suffix, email, password_hash, nickname)
		VALUES ('new', 12345, 'new@example.com', 'new-hash', '新用户')`).Error; err != nil {
		t.Fatal(err)
	}
	var avatar string
	if err := db.Raw("SELECT avatar FROM users WHERE username = 'new'").Scan(&avatar).Error; err != nil || avatar != "" {
		t.Fatalf("new account avatar: %q %v", avatar, err)
	}
	// 重启后重复迁移，已有账号和头像字段都应保留。
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.User{}).Where("avatar = ''").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("repeat migration: count=%d err=%v", count, err)
	}
}
