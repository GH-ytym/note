package calendar

import (
	"context"
	"note/internal/event"
	"time"

	apperrors "note/internal/errors"
	"note/internal/todo"
)

// TodoSource 表示 Calendar 需要的 Todo 查询能力。
type TodoSource interface {
	//实际上就是service那个CalendarOccurrences
	CalendarOccurrences(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]todo.CalendarOccurrence, error)
}

// EventSource 表示 Calendar 需要的 Event 查询能力。
type EventSource interface {
	//实际上就是service实现的那个ListInRange
	ListInRange(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]event.CalendarOccurrence, error)
}

// Result 是 Calendar 汇总后的结果，不是数据库表。
type Result struct {
	Todos  []todo.CalendarOccurrence  `json:"todos"`
	Events []event.CalendarOccurrence `json:"events"`
}

// CalendarService 声明 Calendar 对 Handler 提供的能力。
type CalendarService interface {
	//聚合todo和event的函数
	Get(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) (Result, error)
}

type service struct {
	todos  TodoSource
	events EventSource
}

func NewService(
	todos TodoSource,
	events EventSource,
) CalendarService {
	return &service{
		todos:  todos,
		events: events,
	}
}

func (s *service) Get(
	ctx context.Context,
	from time.Time,
	to time.Time,
) (Result, error) {
	if from.IsZero() ||
		to.IsZero() ||
		!from.Before(to) {
		return Result{}, apperrors.ErrInvalidCalendarRange
	}

	todos, err := s.todos.CalendarOccurrences(ctx, from, to)
	if err != nil {
		return Result{}, err
	}

	events, err := s.events.ListInRange(ctx, from, to)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Todos:  todos,
		Events: events,
	}, nil
}
