package auth

import (
	"strings"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, name := range []string{"小明", "Alice_123", strings.Repeat("中", 20)} {
		if err := validateName(name); err != nil {
			t.Errorf("valid name %q: %v", name, err)
		}
	}
	for _, name := range []string{"a", "NIL", "Null", "undefined", "名字#12345", "名字 ", "名字!", "用户😀", strings.Repeat("中", 21)} {
		if validateName(name) == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	for _, password := range []string{"12345678", strings.Repeat("A", 20), "Demo_12345"} {
		if err := validatePassword(password); err != nil {
			t.Errorf("valid password: %v", err)
		}
	}
	for _, password := range []string{"1234567", strings.Repeat("A", 21), "Demo 12345", "Demo_123!", "密码123456", "Demo_123\n"} {
		if validatePassword(password) == nil {
			t.Error("accepted invalid password")
		}
	}
	email, err := normalizeEmail(" USER@Example.COM ")
	if err != nil || email != "user@example.com" {
		t.Fatal("normalization failed", err)
	}
	for _, email := range []string{"bad", "a b@example.com", "用户@example.com", "User <user@example.com>"} {
		if _, err := normalizeEmail(email); err == nil {
			t.Errorf("accepted invalid email %q", email)
		}
	}
}
