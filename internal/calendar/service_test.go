package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "note/internal/errors"
	"note/internal/event"
	"note/internal/todo"
)

type todoSourceStub struct {
	result []todo.CalendarOccurrence
	err    error
	calls  int
}

func (s *todoSourceStub) CalendarOccurrences(
	_ context.Context,
	_, _ time.Time,
) ([]todo.CalendarOccurrence, error) {
	s.calls++
	return s.result, s.err
}

type eventSourceStub struct {
	result []event.CalendarOccurrence
	err    error
	calls  int
}

func (s *eventSourceStub) ListInRange(
	_ context.Context,
	_, _ time.Time,
) ([]event.CalendarOccurrence, error) {
	s.calls++
	return s.result, s.err
}

func TestGetCombinesTodosAndEvents(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	todos := &todoSourceStub{result: []todo.CalendarOccurrence{{TodoID: 1}}}
	events := &eventSourceStub{result: []event.CalendarOccurrence{{EventID: 2}}}
	service := NewService(todos, events)

	result, err := service.Get(context.Background(), from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(result.Todos) != 1 || result.Todos[0].TodoID != 1 {
		t.Errorf("Todos = %#v", result.Todos)
	}
	if len(result.Events) != 1 || result.Events[0].EventID != 2 {
		t.Errorf("Events = %#v", result.Events)
	}
	if todos.calls != 1 || events.calls != 1 {
		t.Errorf("source calls = todos:%d events:%d, want 1 each", todos.calls, events.calls)
	}
}

func TestGetStopsWhenTodoSourceFails(t *testing.T) {
	from := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	wantErr := errors.New("todo query failed")
	todos := &todoSourceStub{err: wantErr}
	events := &eventSourceStub{}
	service := NewService(todos, events)

	_, err := service.Get(context.Background(), from, from.AddDate(0, 0, 1))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Get() error = %v, want %v", err, wantErr)
	}
	if events.calls != 0 {
		t.Errorf("event source calls = %d, want 0", events.calls)
	}
}

func TestGetRejectsInvalidRange(t *testing.T) {
	start := time.Date(2026, time.September, 9, 0, 0, 0, 0, time.Local)
	service := NewService(&todoSourceStub{}, &eventSourceStub{})

	_, err := service.Get(context.Background(), start, start)
	if !errors.Is(err, apperrors.ErrInvalidCalendarRange) {
		t.Fatalf("Get() error = %v, want %v", err, apperrors.ErrInvalidCalendarRange)
	}
}
