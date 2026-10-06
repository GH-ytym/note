package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"

	"github.com/ncruces/go-sqlite3/gormlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func authTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(sqliteDSN(filepath.Join(t.TempDir(), "auth.db"))), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestRegisterAndLogin(t *testing.T) {
	db := authTestDB(t)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewAuthHandler(auth.NewService(auth.NewGORMRepository(db)), tokens, newTestRefreshStore(t), false)
	r := router.NewWithWeb(nil, nil, nil, nil, h, handler.NewGroupHandler(nil), nil, tokens, "")
	request := func(path string, body map[string]string, status int) []byte {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Note-Request", "1")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s: got %d, want %d: %s", path, res.Code, status, res.Body.String())
		}
		if res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
		if strings.Contains(res.Body.String(), "Demo_12345") || strings.Contains(res.Body.String(), "$2a$") {
			t.Fatal("password leak")
		}
		return res.Body.Bytes()
	}
	body := map[string]string{"name": "小明", "email": "MING@example.com", "password": "Demo_12345"}
	data := request("/api/auth/register", body, 201)
	var first handler.RegisterResponse
	if err := json.Unmarshal(data, &first); err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.Suffix < 10000 || first.Suffix > 99999 {
		t.Fatal("invalid identity")
	}
	var user model.User
	if err := db.First(&user, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "ming@example.com" {
		t.Fatal("email not normalized")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body["password"])); err != nil {
		t.Fatal(err)
	}
	body["email"] = "other@example.com"
	data = request("/auth/register", body, 201)
	var second handler.RegisterResponse
	if err := json.Unmarshal(data, &second); err != nil {
		t.Fatal(err)
	}
	if first.Account == second.Account {
		t.Fatal("duplicate account")
	}
	body["email"] = "ming@EXAMPLE.com"
	request("/api/auth/register", body, 409)
	for _, account := range []string{"MING@example.com", first.Account} {
		data := request("/api/auth/login", map[string]string{"account": account, "password": "Demo_12345"}, 200)
		var login handler.LoginResponse
		if err := json.Unmarshal(data, &login); err != nil {
			t.Fatal(err)
		}
		claims, err := tokens.Parse(login.AccessToken)
		if err != nil {
			t.Fatal(err)
		}
		if claims.UserID != first.ID || login.Account != first.Account {
			t.Fatal("wrong identity")
		}
	}
	request("/api/auth/login", map[string]string{"account": "小明", "password": "Demo_12345"}, 401)
	for _, tc := range []struct{ key, value string }{
		{"name", "nil"}, {"name", "小明!"}, {"name", "小 明"}, {"password", "Demo 12345"},
		{"password", "1234567"}, {"password", "Demo_12345!"}, {"email", "invalid"},
	} {
		body := map[string]string{"name": "小明", "email": "valid@example.com", "password": "Demo_12345"}
		body[tc.key] = tc.value
		request("/api/auth/register", body, 400)
	}
}

func TestAuthLegacyMigration(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			db := authTestDB(t)
			if err := db.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text NOT NULL, password_hash text NOT NULL, nickname text NOT NULL, created_at datetime, updated_at datetime)").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("CREATE UNIQUE INDEX idx_users_username ON users(username)").Error; err != nil {
				t.Fatal(err)
			}
			if populated {
				if err := db.Exec("INSERT INTO users (username,password_hash,nickname) VALUES ('old','hash','old')").Error; err != nil {
					t.Fatal(err)
				}
				if err := migrateAuthSchema(db); err == nil {
					t.Fatal("must stop for existing legacy accounts")
				}
				var count int64
				if err := db.Table("users").Where("username = ?", "old").Count(&count).Error; err != nil || count != 1 {
					t.Fatal("old account lost", err)
				}
				return
			}
			for i := 0; i < 2; i++ {
				if err := migrateAuthSchema(db); err != nil {
					t.Fatal(err)
				}
			}
			if db.Migrator().HasIndex(&model.User{}, "idx_users_username") {
				t.Fatal("old unique index remains")
			}
			for _, user := range []model.User{
				{Username: "小明", Suffix: 12345, Email: "a@example.com", PasswordHash: "hash", Nickname: "小明"},
				{Username: "小明", Suffix: 12346, Email: "b@example.com", PasswordHash: "hash", Nickname: "小明"},
			} {
				if err := db.Create(&user).Error; err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
