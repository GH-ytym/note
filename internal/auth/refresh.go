package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const RefreshTTL = 7 * 24 * time.Hour

var ErrInvalidRefresh = errors.New("刷新令牌无效或已过期")

type RefreshStore struct {
	client *redis.Client
}

func NewRefreshStore(client *redis.Client) *RefreshStore {
	return &RefreshStore{client: client}
}

// 32 字节随机数编码后是 43 个字符。
// 使用 crypto/rand，不能使用 math/rand。
func newRefreshToken() (string, error) {
	var data [32]byte

	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}

// 检查格式，同时拒绝非规范编码。
func validRefreshToken(raw string) bool {
	if len(raw) != 43 {
		return false
	}

	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) != 32 {
		return false
	}

	return base64.RawURLEncoding.EncodeToString(data) == raw
}

// Redis 中不保存原始令牌。
// 加上sha256和固定前缀
func refreshKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "note:refresh:" + hex.EncodeToString(sum[:])
}

// Issue 在账号密码验证成功后调用。
// 一个令牌对应一次登录，因此允许同一账号登录多个设备。
func (s *RefreshStore) Issue(
	ctx context.Context,
	userID uint,
) (string, error) {
	return s.Replace(ctx, "", userID)
}

// 重新登录先写入新会话，再删除旧会话；写入失败不会破坏旧会话。
// 此脚本和 Rotate 一样面向单实例 Redis。
var replaceRefreshScript = redis.NewScript(`
local created = redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2], "NX")
if not created then return 0 end
if ARGV[3] == "1" then redis.call("DEL", KEYS[2]) end
return 1
`)

// 重新输入账密登录，则需要动redis
func (s *RefreshStore) Replace(ctx context.Context, oldRaw string, userID uint) (string, error) {
	if userID == 0 {
		return "", errors.New("user ID must not be zero")
	}

	raw, err := newRefreshToken()
	if err != nil {
		return "", err
	}

	//写入redis
	removeOld := "0"
	if validRefreshToken(oldRaw) {
		removeOld = "1"
	}
	created, err := replaceRefreshScript.Run(
		ctx,
		s.client,
		[]string{refreshKey(raw), refreshKey(oldRaw)},
		strconv.FormatUint(uint64(userID), 10),
		int64(RefreshTTL/time.Second),
		removeOld,
	).Int()
	if err != nil {
		return "", fmt.Errorf("save refresh session: %w", err)
	}
	if created != 1 {
		return "", errors.New("refresh token collision")
	}

	return raw, nil
}

// Lookup 读取刷新令牌对应的用户 ID。
// Redis 不存在此 key，与 Redis 本身故障，是两种不同情况。
func (s *RefreshStore) Lookup(
	ctx context.Context,
	raw string,
) (uint, error) {
	if !validRefreshToken(raw) {
		return 0, ErrInvalidRefresh
	}

	value, err := s.client.Get(ctx, refreshKey(raw)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrInvalidRefresh
	}
	if err != nil {
		return 0, fmt.Errorf("read refresh session: %w", err)
	}

	userID, err := strconv.ParseUint(value, 10, strconv.IntSize)
	if err != nil || userID == 0 {
		return 0, ErrInvalidRefresh
	}

	return uint(userID), nil
}

// 在单实例 Redis 中原子执行：
// 1. 检查旧令牌仍然存在，并且属于期望用户。
// 2. 保存新令牌。
// 3. 删除旧令牌。

// 两个请求同时刷新同一令牌时，只有一个能够成功。
var rotateRefreshScript = redis.NewScript(`
local uid = redis.call("GET", KEYS[1])

if not uid or uid ~= ARGV[1] then
	return 0
end

local created = redis.call(
	"SET",
	KEYS[2],
	uid,
	"EX",
	ARGV[2],
	"NX"
)

if not created then
	return -1
end

redis.call("DEL", KEYS[1])
return 1
`)

// 刷新凭证
func (s *RefreshStore) Rotate(
	ctx context.Context,
	oldRaw string,
	userID uint,
) (string, error) {
	if !validRefreshToken(oldRaw) || userID == 0 {
		return "", ErrInvalidRefresh
	}

	newRaw, err := newRefreshToken()
	if err != nil {
		return "", err
	}

	result, err := rotateRefreshScript.Run(
		ctx,
		s.client,
		[]string{
			refreshKey(oldRaw),
			refreshKey(newRaw),
		},
		strconv.FormatUint(uint64(userID), 10),
		int64(RefreshTTL/time.Second),
	).Int()
	if err != nil {
		return "", fmt.Errorf("rotate refresh session: %w", err)
	}

	switch result {
	case 1:
		return newRaw, nil
	case 0:
		return "", ErrInvalidRefresh
	default:
		return "", errors.New("refresh token collision")
	}
}

// 退出登录需要删除redis记录
// Revoke 是幂等操作：令牌已经不存在，也算退出成功。
func (s *RefreshStore) Revoke(
	ctx context.Context,
	raw string,
) error {
	if !validRefreshToken(raw) {
		return nil
	}

	if err := s.client.Del(ctx, refreshKey(raw)).Err(); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}
