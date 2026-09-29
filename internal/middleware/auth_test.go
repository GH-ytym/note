package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"note/internal/auth"
)

func TestRequireLogin(t *testing.T) {
	secret := "test-only-secret-0123456789abcdef"
	m, err := auth.NewTokenManager(secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := m.Generate(42)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{UserID: 42, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "note", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, header string
		allowed      bool
	}{
		{"missing", "", false},
		{"missing token", "Bearer", false},
		{"wrong scheme", "Basic " + valid, false},
		{"extra field", "Bearer " + valid + " extra", false},
		{"malformed", "Bearer invalid", false},
		{"expired", "Bearer " + expired, false},
		{"valid", "Bearer " + valid, true},
		{"case insensitive scheme", "bearer " + valid, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			called := false
			r.GET("/private", RequireLogin(m), func(c *gin.Context) {
				called = true
				id, exists := c.Get(UserIDKey)
				if !exists || id != uint(42) {
					t.Errorf("wrong identity: %v", id)
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", tc.header)
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			want := http.StatusUnauthorized
			if tc.allowed {
				want = http.StatusNoContent
			}
			if res.Code != want || called != tc.allowed {
				t.Fatalf("status=%d handler called=%v, want status=%d called=%v", res.Code, called, want, tc.allowed)
			}
		})
	}
}
