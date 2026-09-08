package utils

import (
	"testing"
	"time"
)

func TestUniqueDatesPreservesFirstOccurrence(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	first := time.Date(2026, time.September, 1, 8, 0, 0, 0, location)
	duplicate := time.Date(2026, time.September, 1, 18, 0, 0, 0, location)
	second := time.Date(2026, time.September, 2, 8, 0, 0, 0, location)

	dates := UniqueDates([]time.Time{first, duplicate, second})
	if len(dates) != 2 || !dates[0].Equal(first) || !dates[1].Equal(second) {
		t.Fatalf("UniqueDates() = %v", dates)
	}
}
