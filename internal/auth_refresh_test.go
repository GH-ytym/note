package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRefreshStore(t *testing.T) *auth.RefreshStore {
	t.Helper()
	s := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: s.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = c.Close() })
	return auth.NewRefreshStore(c)
}

func TestRefreshRoutes(t *testing.T) {
	db := authTestDB(t)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(auth.NewGORMRepository(db))
	user, err := service.Register(context.Background(), "alice", "alice@example.com", "Demo_12345")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	store := auth.NewRefreshStore(client)
	h := handler.NewAuthHandler(service, tokens, store, true)
	r := router.NewWithWeb(nil, nil, nil, nil, h, nil, tokens, "")
	request := func(path, body string, cookie *http.Cookie, csrf bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Refresh must ignore an expired/invalid access JWT.
		req.Header.Set("Authorization", "Bearer expired")
		if csrf {
			req.Header.Set("X-Note-Request", "1")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		return res
	}
	assertStatus := func(res *httptest.ResponseRecorder, want int) {
		t.Helper()
		if res.Code != want {
			t.Fatalf("got %d want %d: %s", res.Code, want, res.Body.String())
		}
		if res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
	}
	readCookie := func(res *httptest.ResponseRecorder) *http.Cookie {
		t.Helper()
		cookies := res.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies: %v", cookies)
		}
		c := cookies[0]
		if c.Name != "note_refresh" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != int(auth.RefreshTTL/time.Second) {
			t.Fatalf("bad cookie: %+v", c)
		}
		return c
	}
	for _, prefix := range []string{"", "/api"} {
		assertStatus(request(prefix+"/auth/refresh", "", nil, true), 401)
		assertStatus(request(prefix+"/auth/refresh", "", nil, false), 403)
		assertStatus(request(prefix+"/auth/login", "{}", nil, false), 403)
		assertStatus(request(prefix+"/auth/logout", "", nil, false), 403)
		assertStatus(request(prefix+"/auth/logout", "", nil, true), 204)
	}
	login := request("/api/auth/login", `{"account":"alice@example.com","password":"Demo_12345"}`, nil, true)
	assertStatus(login, 200)
	old := readCookie(login)
	// Redis 故障时不清 Cookie，保留重试退出的能力。
	server.SetError("ERR simulated unavailable")
	failedLogout := request("/api/auth/logout", "", old, true)
	assertStatus(failedLogout, 503)
	if len(failedLogout.Result().Cookies()) != 0 {
		t.Fatal("failed logout cleared cookie")
	}
	server.SetError("")
	for _, path := range []string{"/auth/refresh", "/api/auth/refresh"} {
		res := request(path, "", old, true)
		assertStatus(res, 200)
		var body handler.LoginResponse
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		claims, err := tokens.Parse(body.AccessToken)
		if err != nil || claims.UserID != user.ID {
			t.Fatalf("invalid JWT: %v", err)
		}
		next := readCookie(res)
		if next.Value == old.Value {
			t.Fatal("token not rotated")
		}
		replay := request(path, "", old, true)
		assertStatus(replay, 401)
		if len(replay.Result().Cookies()) != 0 {
			t.Fatal("failed refresh must not overwrite winning cookie")
		}
		old = next
	}
	logout := request("/api/auth/logout", "", old, true)
	assertStatus(logout, 204)
	cleared := logout.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge != -1 || cleared[0].Path != "/" || !cleared[0].HttpOnly || !cleared[0].Secure {
		t.Fatalf("bad cleared cookie: %v", cleared)
	}
	assertStatus(request("/auth/refresh", "", old, true), 401)
	assertStatus(request("/auth/logout", "", old, true), 204)
	server.FastForward(auth.RefreshTTL + time.Second)
	assertStatus(request("/auth/refresh", "", old, true), 401)
	raw, err := store.Issue(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.User{}, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertStatus(request("/auth/refresh", "", &http.Cookie{Name: "note_refresh", Value: raw}, true), 401)
	server.Close()
	assertStatus(request("/auth/refresh", "", &http.Cookie{Name: "note_refresh", Value: raw}, true), 503)
}
