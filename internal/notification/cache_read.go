package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var scriptLua = redis.NewScript(readPageLua)

// ReadPage 在同一个 Lua 脚本中读取一页通知及 epoch、revision 等元数据。
// before 为 0 时读取第一页，否则只读 ID 小于 before 的通知；limit 范围为 1 到 100。
// 返回 true 表示这一页可用（包括空列表）；false, nil 表示缓存缺失或需要重建。
// 本方法不重建、不查询数据库；各次分页请求之间仍可能发生业务更新。
//
// 分页读取itmes
func (r *RedisCache) ReadPage(
	ctx context.Context,
	uid uint,
	before uint,
	limit uint,
) (Page, bool, error) {
	//准备key和参数
	keys, args, err := pgArgs(uid, before, limit)
	if err != nil {
		return Page{}, false, err
	}

	//执行lua
	values, err := scriptLua.Run(
		ctx,
		r.client,
		keys,
		args...,
	).Slice()
	if err != nil {
		return Page{}, false, fmt.Errorf("read notification page: %w", err)
	}

	//检查values
	//如果正常的话，应该返回{"ok", epoch, rev, unread, nextID, cards}
	if len(values) == 0 {
		return Page{}, false, fmt.Errorf("read notification page: empty result")
	}

	s0, ok := values[0].(string)
	if !ok {
		return Page{}, false, fmt.Errorf("invalid page status")
	}
	switch s0 {
	case "missing", "rebuild":
		return Page{}, false, nil
	case "ok": //过
	default:
		return Page{}, false, fmt.Errorf("invalid page status")
	}

	//"ok"会有总共6个东西
	if len(values) != 6 {
		return Page{}, false, fmt.Errorf("invalid page result length")
	}

	//拿出epoch、revision、unread、next
	var fields [4]string

	for i := range fields {
		text, ok := values[i+1].(string)
		if !ok {
			return Page{}, false, fmt.Errorf(
				"invalid page field at position %d", i+1,
			)
		}
		fields[i] = text
	}

	rev, err := strconv.ParseUint(fields[1], 10, 63)
	if err != nil {
		return Page{}, false, fmt.Errorf("parse page revision: %w", err)
	}

	unread, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return Page{}, false, fmt.Errorf("parse page unread count: %w", err)
	}

	next, err := strconv.ParseUint(fields[3], 10, strconv.IntSize)
	if err != nil {
		return Page{}, false, fmt.Errorf("parse page cursor: %w", err)
	}

	if next > uint64(1<<53-1) {
		return Page{}, false, fmt.Errorf("invalid page cursor")
	}

	//取出card json
	rows, ok := values[5].([]any)
	if !ok {
		return Page{}, false, fmt.Errorf("invalid page cards")
	}

	page := Page{
		Epoch:       fields[0],
		Revision:    rev,
		UnreadCount: unread,
		Next:        uint(next),
		Items:       make([]Card, 0, len(rows)),
	}

	//变成card类型的，塞进items
	for _, row := range rows {
		data, ok := row.(string)
		if !ok {
			return Page{}, false, fmt.Errorf("invalid page card data")
		}

		var card Card
		if err := json.Unmarshal([]byte(data), &card); err != nil {
			return Page{}, false, fmt.Errorf("decode page card: %w", err)
		}

		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return Page{}, false, fmt.Errorf("invalid page card ID")
		}

		page.Items = append(page.Items, card)
	}

	return page, true, nil
}
