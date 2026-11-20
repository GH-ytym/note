package notification

import (
	"context"
	"fmt"
	"log"
	"note/internal/model"
	"time"
)

type Deliverer interface {
	Deliver(ctx context.Context, task model.Outbox) (bool, error)
}

// PublishPending 按 ID 顺序串行处理一批未发布的 Outbox，每条完成后单独标记。
// 遇到错误或 Deliver 返回 false 就结束本轮，未标记的任务会被后续轮次再次查到。
// 每条任务交给 Deliverer 分流，通知和日历各自完成投递。
func PublishPending(
	ctx context.Context,
	repo Repository,
	delivery Deliverer,
) error {
	//查询未投递的任务
	tasks, err := repo.ListPendingOutbox(ctx)
	if err != nil {
		return fmt.Errorf("ListPendingOutbox: %w", err)
	}

	for _, t := range tasks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// 业务事务已提交，现在将 Outbox 中的变化投递到 Redis。
		//一定是先保存到sql再publish，这样接收者就可以继续操作
		//否则发送滚木了自己都不知道
		ok, err := delivery.Deliver(ctx, t)
		if err != nil {
			return fmt.Errorf("Deliver: %w", err)
		}
		if !ok {
			//本轮直接结束，反正会有下一轮
			return nil
		}

		//redis成功了才能把任务标记投递成功
		if err := repo.MarkOutboxPublished(ctx, t.ID, time.Now()); err != nil {
			return fmt.Errorf("mark outbox %d: %w", t.ID, err)
		}
	}
	return nil
}

// RunOutboxWorker 在 ctx 取消之前持续投递，每轮串行执行。
// 第一次立即投递，之后通过定时器等待下一轮。
// 定时器间隔为一秒，每轮共用五秒超时；本轮失败只记录日志，后续轮次继续处理。
func RunOutboxWorker(
	ctx context.Context,
	repo Repository,
	delivery Deliverer,
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
		roundCtx, roundCancel := context.WithTimeout(
			ctx,
			5*time.Second,
		)
		err := PublishPending(roundCtx, repo, delivery)
		roundCancel()

		// 服务关闭时正常退出，不把取消记录成投递失败。
		if ctx.Err() != nil {
			return
		}
		// 失败只结束当前一轮；未标记成功的任务留给下一轮重试。
		if err != nil {
			log.Printf("publish pending outbox failed: %v", err)
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
