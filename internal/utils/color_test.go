package utils

import "testing"

func TestRandomColorDoesNotRepeatImmediately(t *testing.T) {
	previous := RandomColor()
	for range 64 {
		next := RandomColor()
		if next == previous {
			t.Fatalf("random color repeated immediately: %s", next)
		}
		previous = next
	}
}

func TestNormalizeHexColor(t *testing.T) {
	color, ok := NormalizeHexColor("#a1b2c3")
	if !ok || color != "#A1B2C3" {
		t.Fatalf("NormalizeHexColor() = %q, %v", color, ok)
	}

	if _, ok := NormalizeHexColor("yellow"); ok {
		t.Fatal("invalid color was accepted")
	}
}
