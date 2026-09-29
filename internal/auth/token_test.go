package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenManagerRoundTrip(t *testing.T) {
	m, err := NewTokenManager("test-only-secret-0123456789abcdef", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{1, 42} {
		token, err := m.Generate(id)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := m.Parse(token)
		if err != nil {
			t.Fatal(err)
		}
		if claims.UserID != id || claims.Issuer != "note" || claims.IssuedAt == nil || claims.ExpiresAt == nil {
			t.Fatalf("unexpected claims: %+v", claims)
		}
		if claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 15*time.Minute {
			t.Fatal("incorrect token lifetime")
		}
	}
	if _, err := m.Generate(0); err == nil {
		t.Fatal("accepted zero user ID")
	}
}

func TestTokenManagerRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		secret string
		ttl    time.Duration
	}{
		{"short", time.Minute},
		{strings.Repeat("s", 32), 0},
		{strings.Repeat("s", 32), -time.Minute},
	} {
		if _, err := NewTokenManager(tc.secret, tc.ttl); err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
}

func TestTokenManagerRejectsInvalidTokens(t *testing.T) {
	secret := "test-only-secret-0123456789abcdef"
	m, err := NewTokenManager(secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wrong key", "wrong algorithm", "wrong issuer", "expired", "missing expiry", "future issued at", "future not before", "zero user", "tampered payload", "malformed"} {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			claims := Claims{UserID: 42, RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "note", IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}}
			var method jwt.SigningMethod = jwt.SigningMethodHS256
			key := []byte(secret)
			switch name {
			case "wrong key":
				key = []byte(strings.Repeat("x", 32))
			case "wrong algorithm":
				method = jwt.SigningMethodHS384
			case "wrong issuer":
				claims.Issuer = "other"
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Hour))
			case "missing expiry":
				claims.ExpiresAt = nil
			case "future issued at":
				claims.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour))
			case "future not before":
				claims.NotBefore = jwt.NewNumericDate(now.Add(time.Hour))
			case "zero user":
				claims.UserID = 0
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			if name == "tampered payload" {
				parts := strings.Split(raw, ".")
				payload, err := base64.RawURLEncoding.DecodeString(parts[1])
				if err != nil {
					t.Fatal(err)
				}
				parts[1] = base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(payload), `"user_id":42`, `"user_id":99`, 1)))
				raw = strings.Join(parts, ".")
			}
			if name == "malformed" {
				raw = "not-a-jwt"
			}
			if claims, err := m.Parse(raw); err == nil || claims != nil {
				t.Fatalf("invalid token accepted: claims=%+v err=%v", claims, err)
			}
		})
	}
}
