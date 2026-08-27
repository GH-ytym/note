// Package retry provides context-aware retry behavior for safe operations.
package retry

import (
	"context"
	"time"
)

const (
	maxAttempts  = 3
	initialDelay = 200 * time.Millisecond
)

// Do is a general retrying function
func Do(ctx context.Context, fn func() error) error {
	var lastErr error
	delay := initialDelay
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		//先查一次ctx
		if err := ctx.Err(); err != nil {
			return err
		}

		//执行一次
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		//再查一次ctx
		if err := ctx.Err(); err != nil {
			return err
		}

		//到次数了不再重试
		if attempt == maxAttempts {
			break
		}

		//指数增长
		select {
		case <-time.After(delay):
			delay *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
