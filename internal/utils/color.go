package utils

import (
	cryptorand "crypto/rand"
	"math/big"
	"regexp"
	"strings"
	"sync"
)

var (
	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	eventColors     = []string{
		"#F3B51B", "#F47C48", "#E95B78", "#A879F2",
		"#5B8DEF", "#35B7A0", "#82B94B", "#D4A373",
	}
	randomColorMu        sync.Mutex
	lastRandomColorIndex = -1
)

// NormalizeHexColor validates a six-digit hex color and normalizes its case.
func NormalizeHexColor(value string) (string, bool) {
	if !hexColorPattern.MatchString(value) {
		return "", false
	}
	return strings.ToUpper(value), true
}

// RandomColor uses the system random source and avoids an immediate repeat.
func RandomColor() string {
	randomColorMu.Lock()
	defer randomColorMu.Unlock()

	if len(eventColors) == 1 {
		return eventColors[0]
	}

	choiceCount := len(eventColors)
	if lastRandomColorIndex >= 0 {
		choiceCount--
	}
	randomValue, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(choiceCount)))
	if err != nil {
		lastRandomColorIndex = (lastRandomColorIndex + 1) % len(eventColors)
		return eventColors[lastRandomColorIndex]
	}

	nextIndex := int(randomValue.Int64())
	if lastRandomColorIndex >= 0 && nextIndex >= lastRandomColorIndex {
		nextIndex++
	}
	lastRandomColorIndex = nextIndex
	return eventColors[nextIndex]
}
