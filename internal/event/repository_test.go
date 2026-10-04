package event

import (
	"context"
	"note/internal/model"
	"testing"
	"time"
)

func TestPatchTimeWithMixedOffsets(t *testing.T) {
	db, owner, group := eventRepositoryFixture(t)
	repo := NewGORMRepository(db)
	ctx := context.Background()
	start, _ := time.Parse(time.RFC3339, "2026-09-12T10:00:00+08:00")
	// Different offsets are valid input and must never trip the SQLite text CHECK.
	item := model.Event{GroupID: group.ID, CreatorID: owner.ID, Title: "drag", StartsAt: start, EndsAt: start.Add(2 * time.Hour).UTC(), RepeatMode: model.RepeatOnce}
	if err := repo.Create(ctx, &item); err != nil {
		t.Fatal(err)
	}
	end := start.Add(150 * time.Minute).UTC()
	updated, err := NewService(repo).Patch(ctx, item.ID, owner.ID, PatchCommand{Version: item.Version, EndsAt: &end})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, item.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.EndsAt.Equal(end) || !got.StartsAt.Equal(start) || got.Version != updated.Version {
		t.Fatalf("incorrect saved endpoints: %+v", got)
	}
	shiftedStart := start.Add(30 * time.Minute).UTC()
	_, err = NewService(repo).Patch(ctx, item.ID, owner.ID, PatchCommand{Version: got.Version, StartsAt: &shiftedStart})
	if err != nil {
		t.Fatal(err)
	}
	got, err = repo.Get(ctx, item.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.StartsAt.Equal(shiftedStart) || !got.EndsAt.Equal(end) {
		t.Fatalf("incorrect start patch: %+v", got)
	}
}
