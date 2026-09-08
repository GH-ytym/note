package utils

import "time"

// UniqueDates removes duplicate calendar dates while preserving input order.
func UniqueDates(values []time.Time) []time.Time {
	dates := make([]time.Time, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, date := range values {
		key := date.Format(time.DateOnly)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		dates = append(dates, date)
	}

	return dates
}
