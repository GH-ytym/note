package notification

import (
	"context"
	"fmt"
	"log"
	"time"
)

func PublishPending(
	ctx context.Context,
	repo Repository,
	publisher *RedisPublisher,
) error {
	//查询未投递的任务
	tasks, err := repo.ListPendingOutbox(ctx)
	if err != nil {
		return fmt.Errorf("ListPendingOutbox: %w", err)
	}

	for _, t := range tasks {
		//保存历史并广播
		//一定是先保存到sql再publish，这样接收者就可以继续操作
		//否则发送滚木了自己都不知道
		if _, err := publisher.Publish(ctx, t); err != nil {
			return fmt.Errorf("publish outbox %d: %w", t.ID, err)
		}

		//redis成功了，才算标记投递成功
		if err := repo.MarkOutboxPublished(ctx, t.ID, time.Now()); err != nil {
			return fmt.Errorf("mark outbox %d: %w", t.ID, err)
		}
	}
	return nil
}

// 在 ctx 取消之前持续投递，每轮串行执行。
// 第一次立即投递，之后通过定时器等待下一轮。
func RunOutboxWorker(
	ctx context.Context,
	repo Repository,
	publisher *RedisPublisher,
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		// 停止后不再开始新一轮投递。
		if ctx.Err() != nil {
			return
		}

		// 每轮都要重新创建 context；上一轮结束后就释放它。
		// PublishPending 必须在循环里面，才能持续处理后来新增的任务。
		roundCtx, roundCancel := context.WithTimeout(ctx, 5*time.Second)
		err := PublishPending(roundCtx, repo, publisher)
		roundCancel()

		// 服务关闭时正常退出，不把取消记录成投递失败。
		if ctx.Err() != nil {
			return
		}
		// 失败只结束当前一轮；未标记成功的任务留给下一轮重试。
		if err != nil {
			log.Printf("publish pending notifications failed: %v", err)
			//不return
		}

		// 没有任务或本轮失败时也等待，避免不停查询或重试。
		select {
		case <-ticker.C: //不return，这里只等待1秒

		case <-ctx.Done():
			return
		}
	}
}
