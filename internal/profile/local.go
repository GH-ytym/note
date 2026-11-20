package profile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

var avatarKey = regexp.MustCompile(`^avatars/[1-9][0-9]*/[a-f0-9]{32}\.jpg$`)
var mediaBase = regexp.MustCompile(`^/[a-zA-Z0-9/_-]+$`)

// LocalStore 将头像文件保存在专用目录，URL 保留反向代理的部署前缀。
type LocalStore struct {
	root *os.Root
	base string
}

func NewStoreFromEnv() (Store, error) {
	if dir := os.Getenv("NOTE_AVATAR_DIR"); dir != "" {
		base := os.Getenv("NOTE_AVATAR_URL")
		if base == "" {
			base = "/api/media"
		}
		return NewLocalStore(dir, base)
	}
	return NewCOSStoreFromEnv()
}

func NewLocalStore(dir, base string) (*LocalStore, error) {
	base = strings.TrimRight(base, "/")
	if !mediaBase.MatchString(base) || strings.HasPrefix(base, "//") || path.Clean(base) != base {
		return nil, fmt.Errorf("invalid avatar URL path")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &LocalStore{root, base}, nil
}
func (s *LocalStore) Close() error { return s.root.Close() }

func (s *LocalStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !avatarKey.MatchString(key) {
		return "", fmt.Errorf("invalid avatar key")
	}
	if err := s.root.MkdirAll(path.Dir(key), 0700); err != nil {
		return "", err
	}
	// 随机文件名不覆盖旧图，临时文件完成后才对读取路由可见。
	tmp := key + ".tmp"
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer s.root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closed := f.Close()
	if err != nil {
		return "", err
	}
	if closed != nil {
		return "", closed
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := s.root.Rename(tmp, key); err != nil {
		return "", err
	}
	return s.base + "/" + key, nil
}

func (s *LocalStore) Delete(ctx context.Context, url string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, ok := strings.CutPrefix(url, s.base+"/")
	// 不删除其他存储服务的图片，也不接受目录或任意文件路径。
	if !ok || !avatarKey.MatchString(key) {
		return nil
	}
	err := s.root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ServeHTTP 只允许读取已处理的头像；不列目录，不暴露数据库等文件。
// 头像与此前 COS 公共图片一致：知道随机 URL 即可读取，上传仍需认证。
func (s *LocalStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")
	if !avatarKey.MatchString(key) {
		http.NotFound(w, r)
		return
	}
	f, err := s.root.Open(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, path.Base(key), info.ModTime(), f)
}
