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
	"note/internal/todo"

	"github.com/ncruces/go-sqlite3/gormlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestAuthLogin(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(sqliteDSN(filepath.Join(t.TempDir(), "login.db"))), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("Test_12345"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "alice", Suffix: 12345, Email: "alice@example.com", PasswordHash: string(hash), Nickname: "Alice"}).Error; err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewAuthHandler(auth.NewService(auth.NewGORMRepository(db)), tokens, newTestRefreshStore(t), false)
	th := handler.NewTodoHandler(todo.NewService(todo.NewGORMRepository(db)))
	r := router.NewWithWeb(th, nil, nil, nil, h, handler.NewGroupHandler(nil), tokens, "")
	for _, tc := range []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"success", "/api/auth/login", `{"account":"alice#12345","password":"Test_12345"}`, http.StatusOK},
		{"root route", "/auth/login", `{"account":"alice#12345","password":"Test_12345"}`, http.StatusOK},
		{"wrong password", "/api/auth/login", `{"account":"alice#12345","password":"wrong"}`, http.StatusUnauthorized},
		{"unknown user", "/api/auth/login", `{"account":"missing#12345","password":"wrong"}`, http.StatusUnauthorized},
		{"missing password", "/api/auth/login", `{"account":"alice#12345"}`, http.StatusBadRequest},
		{"invalid JSON", "/api/auth/login", `{`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Note-Request", "1")
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", res.Code, tc.status, res.Body.String())
			}
			if strings.Contains(res.Body.String(), string(hash)) || strings.Contains(strings.ToLower(res.Body.String()), "password") {
				t.Fatal("response leaks password data")
			}
			if tc.status == http.StatusOK && !strings.Contains(res.Body.String(), `"name":"alice"`) {
				t.Fatal("response missing user")
			}
			if tc.status == http.StatusOK {
				var login handler.LoginResponse
				if err := json.Unmarshal(res.Body.Bytes(), &login); err != nil {
					t.Fatal(err)
				}
				if login.TokenType != "Bearer" || login.AccessToken == "" {
					t.Fatal("response missing bearer token")
				}
				claims, err := tokens.Parse(login.AccessToken)
				if err != nil || claims.UserID != login.ID || login.ID == 0 {
					t.Fatalf("token does not identify logged-in user: claims=%+v, err=%v", claims, err)
				}
				for _, path := range []string{"/todos", "/api/todos"} {
					request := httptest.NewRequest(http.MethodGet, path+"?page=1&page_size=10", nil)
					request.Header.Set("Authorization", "Bearer "+login.AccessToken)
					response := httptest.NewRecorder()
					r.ServeHTTP(response, request)
					if response.Code != http.StatusOK {
						t.Fatalf("login token rejected by %s: %d %s", path, response.Code, response.Body.String())
					}
				}
			}
		})
	}
}
