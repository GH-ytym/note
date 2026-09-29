// Package retry provides context-aware retry behavior for safe operations.
package retry

import (
	"context"
	"errors"
	"time"
)

const (
	maxAttempts  = 3
	initialDelay = 200 * time.Millisecond
)

// Options 控制重试策略；ShouldRetry 为 nil 时重试所有错误。
type Options struct {
	MaxAttempts  int
	InitialDelay time.Duration
	ShouldRetry  func(error) bool
}

// Do 执行闭包。省略 options 时保留原来的三次、指数退避行为。
func Do(ctx context.Context, fn func() error, options ...Options) error {
	if len(options) > 1 {
		return errors.New("retry: at most one Options is allowed")
	}
	config := Options{MaxAttempts: maxAttempts, InitialDelay: initialDelay}
	if len(options) == 1 {
		config = options[0]
	}
	if config.MaxAttempts < 1 || config.InitialDelay < 0 {
		return errors.New("retry: invalid attempts or delay")
	}
	var lastErr error
	delay := config.InitialDelay
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		//先查一次ctx
		if err := ctx.Err(); err != nil {
			return err
		}

		//执行一次
		lastErr = fn()
		//成功了就直接返回
		if lastErr == nil {
			return nil
		}
		//配置条件写了不再重试的话就退出了
		if config.ShouldRetry != nil && !config.ShouldRetry(lastErr) {
			return lastErr
		}

		//再查一次ctx
		if err := ctx.Err(); err != nil {
			return err
		}

		//到次数了不再重试
		if attempt == config.MaxAttempts {
			break
		}
		if delay == 0 {
			continue
		}

		//指数增长
		select {
		case <-time.After(delay):
			// 防止 time.Duration 在长重试序列中溢出。
			if delay <= time.Duration(1<<63-1)/2 {
				delay *= 2
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
