package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var install = redis.NewScript(installSnapshotLua)

// 使用本次重建资格，尝试安装完整快照。
// inbox 必须是取得资格后，为 l.userID 加载的数据库快照。
func (r *RedisCache) installSnapshot(
	ctx context.Context,
	l *lease,
	inbox Inbox,
) (bool, error) {
	if l == nil || l.userID == 0 || l.token == "" {
		return false, errors.New("invalid rebuild lease")
	}

	if inbox.Revision > uint64(1<<63-1) {
		return false, errors.New("revision exceeds SQLite integer range")
	}
	if inbox.UnreadCount < 0 {
		return false, errors.New("unread count cannot be negative")
	}

	// 1. 将开始重建时记录的旧状态编码为 JSON。
	if l.old == nil {
		return false, errors.New("missing old state observation")
	}
	if !l.exist && len(l.old) != 0 {
		return false, errors.New("unexpected fields for missing state")
	}

	old, err := json.Marshal(l.old)
	if err != nil {
		return false, fmt.Errorf("encode old state: %w", err)
	}

	exist := "0"
	if l.exist {
		exist = "1"
	}

	// 2. 为本次安装生成新的缓存代次。
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return false, fmt.Errorf("create cache epoch: %w", err)
	}
	epoch := hex.EncodeToString(buf[:])

	// 3. 准备脚本会访问的 key。
	k := notificationCacheKeys(l.userID)

	keys := []string{
		rebuildKey(l.userID), // KEYS[1]：重建资格
		k.state,              // KEYS[2]：缓存状态
		k.items,              // KEYS[3]：通知卡片
		k.order,              // KEYS[4]：排序
		k.history,            // KEYS[5]：变化历史
		k.live,               // KEYS[6]：广播频道
	}

	// 4. 固定参数。
	args := []any{
		l.token,                                  // ARGV[1]
		exist,                                    // ARGV[2]
		string(old),                              // ARGV[3]
		epoch,                                    // ARGV[4]
		strconv.FormatUint(inbox.Revision, 10),   // ARGV[5]
		strconv.FormatInt(inbox.UnreadCount, 10), // ARGV[6]
		len(inbox.Items),                         // ARGV[7]
	}

	// 5. 从第 8 个参数开始，追加每张卡片的 ID 和 JSON。
	seen := make(map[uint]bool, len(inbox.Items))

	for _, card := range inbox.Items {
		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return false, fmt.Errorf("invalid notification ID: %d", card.ID)
		}
		if seen[card.ID] {
			return false, fmt.Errorf("duplicate notification ID: %d", card.ID)
		}
		seen[card.ID] = true

		data, err := json.Marshal(card)
		if err != nil {
			return false, fmt.Errorf("encode notification card: %w", err)
		}

		args = append(
			args,
			strconv.FormatUint(uint64(card.ID), 10),
			string(data),
		)
	}

	// 执行安装脚本：1 表示完成，0 表示资格失效或状态冲突。
	n, err := install.Run(ctx, r.client, keys, args...).Int()
	if err != nil {
		return false, fmt.Errorf("install snapshot: %w", err)
	}

	switch n {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected install result: %d", n)
	}
}
