package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type ApplyResult string

const (
	ApplyApplied ApplyResult = "applied" // 已更新
	ApplyIgnored ApplyResult = "ignored" // 重复或旧消息
	ApplyMissing ApplyResult = "missing" // 缓存不存在
	ApplyGap     ApplyResult = "gap"     // 中间缺少版本
)

var applyChangeScript = redis.NewScript(applyChangeLua)

func (r *RedisCache) ApplyChange(
	ctx context.Context,
	userID uint,
	change Change,
) (ApplyResult, error) {
	if userID == 0 {
		return "", fmt.Errorf("userID cannot be zero")
	}
	if change.Revision == 0 || change.Revision > uint64(1<<63-1) {
		return "", fmt.Errorf("invalid change revision")
	}
	if change.UnreadCount < 0 {
		return "", fmt.Errorf("unread count cannot be negative")
	}

	// JSON 中输出[]  免得出现null
	if change.Upserts == nil {
		change.Upserts = []Card{}
	}
	if change.RemovedIDs == nil {
		change.RemovedIDs = []uint{}
	}

	payload, err := json.Marshal(change)
	if err != nil {
		return "", fmt.Errorf("encode notification change: %w", err)
	}

	// 固定参数占前5个位置
	args := []any{
		strconv.FormatUint(change.Revision, 10),
		strconv.FormatUint(change.Revision-1, 10),
		strconv.FormatInt(change.UnreadCount, 10),
		len(change.Upserts),
		string(payload),
	}

	// 每张卡片追加两个参数
	//ID+完整 Card JSON
	for _, card := range change.Upserts {
		// ID 被用作 ZSet 分数，限制在整数可精确表示的范围。
		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return "", fmt.Errorf(
				"invalid notification ID: %d", card.ID,
			)
		}

		data, err := json.Marshal(card)
		if err != nil {
			return "", fmt.Errorf(
				"encode notification card: %w", err,
			)
		}

		args = append(
			args,
			strconv.FormatUint(uint64(card.ID), 10),
			string(data),
		)
	}

	// 删除id
	for _, id := range change.RemovedIDs {
		if id == 0 || uint64(id) > uint64(1<<53-1) {
			return "", fmt.Errorf(
				"invalid removed notification ID: %d", id,
			)
		}

		args = append(args, strconv.FormatUint(uint64(id), 10))
	}

	keys := notificationCacheKeys(userID)

	value, err := applyChangeScript.Run(
		ctx,
		r.client,
		[]string{
			keys.state,
			keys.items,
			keys.order,
			keys.history,
			keys.live,
		},
		args...,
	).Text()
	if err != nil {
		return "", fmt.Errorf("apply notification change: %w", err)
	}

	result := ApplyResult(value)
	switch result {
	case ApplyApplied, ApplyIgnored, ApplyMissing, ApplyGap:
		return result, nil
	default:
		return "", fmt.Errorf("unexpected apply result: %q", value)
	}
}
