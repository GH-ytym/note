package notification

import (
	"context"
	"fmt"
	"note/internal/model"
)

// 给task分流
type Dispatcher struct {
	note Deliverer
	cal  Deliverer
}

func NewDispatcher(note, cal Deliverer) *Dispatcher {
	return &Dispatcher{
		note: note,
		cal:  cal,
	}
}

func (d *Dispatcher) Deliver(
	ctx context.Context,
	task model.Outbox,
) (bool, error) {
	switch task.Name {
	case "notification.change":
		return d.note.Deliver(ctx, task)

	case "cldr":
		return d.cal.Deliver(ctx, task)

	default:
		return false, fmt.Errorf(
			"unsupported outbox event: %q",
			task.Name,
		)
	}
}
