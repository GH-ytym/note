package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var scriptLua = redis.NewScript(readPageLua)

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
