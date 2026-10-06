package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
)

func TestAuthenticationRouteCoverage(t *testing.T) {
	m, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// Nil handlers deliberately make any accidental execution fail the test.
	r := NewWithWeb(nil, nil, nil, nil, nil, nil, nil, m, "")
	for _, prefix := range []string{"", "/api"} {
		for _, route := range []struct{ method, path string }{
			{"GET", "/notifications"},
			{"POST", "/groups"},
			{"GET", "/groups/1/members"}, {"POST", "/groups/1/dismiss"},
			{"GET", "/calendar"}, {"GET", "/groups/1/search/todos"}, {"GET", "/groups/1/search/events"}, {"GET", "/groups/1/search/all"},
			{"POST", "/todos"}, {"GET", "/groups/1/todos"}, {"GET", "/todos/1"}, {"PATCH", "/todos/1"},
			{"PATCH", "/todos/1/occurrences/2026-09-26"}, {"DELETE", "/todos/1"},
			{"GET", "/todos/1/occurrences/2026-09-26/completions"},
			{"POST", "/events"}, {"GET", "/events/1"}, {"PATCH", "/events/1"},
			{"GET", "/groups/1/events"}, {"DELETE", "/events/1"}, {"PATCH", "/events/1/members"},
		} {
			t.Run(route.method+prefix+route.path, func(t *testing.T) {
				for _, header := range []string{"", "Bearer invalid"} {
					req := httptest.NewRequest(route.method, prefix+route.path, nil)
					req.Header.Set("Authorization", header)
					res := httptest.NewRecorder()
					r.ServeHTTP(res, req)
					if res.Code != http.StatusUnauthorized {
						t.Fatalf("got %d: %s", res.Code, res.Body.String())
					}
				}
			})
		}
		for _, path := range []string{"/ping", "/auth/login", "/auth/register"} {
			t.Run("public"+prefix+path, func(t *testing.T) {
				method, want := http.MethodPost, http.StatusBadRequest
				if path == "/ping" {
					method, want = http.MethodGet, http.StatusOK
				}
				// Invalid JSON reaches parameter validation without using a service.
				req := httptest.NewRequest(method, prefix+path, strings.NewReader("{"))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Note-Request", "1")
				res := httptest.NewRecorder()
				r.ServeHTTP(res, req)
				if res.Code != want {
					t.Fatalf("got %d, want %d: %s", res.Code, want, res.Body.String())
				}
			})
		}
	}
}
