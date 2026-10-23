package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const rebuildTTL = 5 * time.Second

// 保存本次重建资格（唯一redis key），以及取得资格时的redis state
type lease struct {
	userID uint
	token  string

	//我们希望在取得资格的那个时刻看到redis有啥
	//取得资格时，当时state是否存在
	exist bool

	//取得资格时，当时state的全部字段
	old map[string]string
}

func rebuildKey(userID uint) string {
	//redis key
	return fmt.Sprintf(
		"note:notifications:v2:{%d}:rebuild",
		userID,
	)
}

var begin = redis.NewScript(beginRebuildLua)

var release = redis.NewScript(endRebuildLua)

func (r *RedisCache) beginRebuild(ctx context.Context, uid uint) (*lease, error) {
	if uid == 0 {
		return nil, errors.New("invalid user id")
	}

	//生成随机16位token，转化为32个16进制字符
	var t [16]byte
	if _, err := rand.Read(t[:]); err != nil {
		return nil, fmt.Errorf(
			"create rebuild token: %w",
			err,
		)
	}
	token := hex.EncodeToString(t[:])

	// 获取缓存key
	k := notificationCacheKeys(uid)

	// 组合两个key
	keys := []string{
		rebuildKey(uid), // KEYS[1] rebuild
		k.state,         // KEYS[2] state
	}

	// 丢进lua
	args := []any{
		token,
		rebuildTTL.Milliseconds(),
	}

	res, err := begin.Run(
		ctx,
		r.client,
		keys,
		args...,
	).Slice()
	if err != nil {
		return nil, fmt.Errorf("begin rebuild: %w", err)
	}

	//检查第一项
	if len(res) == 0 {
		return nil, errors.New("result empty")
	}
	status, ok := res[0].(string)
	if !ok {
		return nil, errors.New("invalid rebuild status")
	}
	if status == "busy" && len(res) == 1 {
		return nil, nil
	}
	if status != "ok" || len(res) != 3 {
		return nil, errors.New("invalid rebuild result")
	}

	// 5. 第二项表示取得资格时 state 是否存在。
	exist, ok := res[1].(string)
	if !ok || (exist != "0" && exist != "1") {
		return nil, errors.New("invalid state existence flag")
	}

	// 6. 第三项是 HGETALL 返回的字段、值列表。
	fields, ok := res[2].([]any)
	if !ok || len(fields)%2 != 0 {
		return nil, errors.New("invalid old state fields")
	}

	if exist == "0" && len(fields) != 0 {
		return nil, errors.New("unexpected fields for missing state")
	}

	old := make(map[string]string, len(fields)/2)

	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			return nil, errors.New("invalid old state key")
		}

		val, ok := fields[i+1].(string)
		if !ok {
			return nil, errors.New("invalid old state value")
		}

		old[key] = val
	}

	// 7. 确认取得资格并解析完成后，组装 lease。
	return &lease{
		userID: uid,
		token:  token,
		exist:  exist == "1",
		old:    old,
	}, nil

}

func (r *RedisCache) endRebuild(
	ctx context.Context,
	l *lease,
) (bool, error) {
	if l == nil || l.userID == 0 || l.token == "" {
		return false, errors.New("invalid lease")
	}

	//组装lua参数
	keys := []string{
		rebuildKey(l.userID),
	}
	//不用管ttl，redis会管
	args := []any{
		l.token,
	}

	//跑lua
	n, err := release.Run(
		ctx,
		r.client,
		keys,
		args...,
	).Int()
	if err != nil {
		return false, fmt.Errorf("release rebuild lease: %w", err)
	}

	return n == 1, nil
}
