package event

import "time"

// CreateCommand is the validated input passed from the HTTP layer to Service.
type CreateCommand struct {
	Title    string
	Content  *string
	Color    string
	StartsAt time.Time
	EndsAt   time.Time
}
