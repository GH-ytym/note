package group

import (
	"context"
	"errors"
	apperrors "note/internal/errors"
	"note/internal/model"
	"testing"
)

type collisionRepository struct {
	Repository
	calls     int
	fail      error
	remaining int
}

func (r *collisionRepository) Create(_ context.Context, item *model.Group) error {
	r.calls++
	if item.ID != 0 || item.Members[0].GroupID != 0 || len(item.Code) != 6 {
		panic("retry reused rolled back state")
	}
	if r.remaining > 0 {
		r.remaining--
		item.ID = 99
		item.Members[0].GroupID = 99
		return r.fail
	}
	item.ID = 1
	return nil
}
func TestCreateRetriesOnlyGroupCodeCollisions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		fail            error
		remaining, want int
		success         bool
	}{
		{"collision", apperrors.ErrGroupCodeConflict, 1, 2, true},
		{"bounded", apperrors.ErrGroupCodeConflict, 10, 5, false},
		{"database error", errors.New("database unavailable"), 10, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &collisionRepository{fail: tc.fail, remaining: tc.remaining}
			_, err := NewService(repo).Create(context.Background(), 1, "test")
			if repo.calls != tc.want || (err == nil) != tc.success {
				t.Fatalf("calls=%d err=%v", repo.calls, err)
			}
		})
	}
}
