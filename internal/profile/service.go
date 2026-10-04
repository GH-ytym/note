package profile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"note/internal/model"
	"time"

	"golang.org/x/image/draw"
)

const MaxUploadBytes = 5 << 20

var ErrInvalidImage = errors.New("请选择 JPEG 或 PNG 图片，大小不超过 5MB，宽高不超过 4096 像素")
var ErrStorageUnavailable = errors.New("头像存储暂未配置或不可用")

// Store 可替换为其他云对象存储；没有本地文件存储实现。
type Store interface {
	Put(context.Context, string, []byte) (string, error)
	Delete(context.Context, string) error
}
type Service struct {
	repo  Repository
	store Store
}

func NewService(repo Repository, store Store) *Service { return &Service{repo, store} }
func (s *Service) Get(ctx context.Context, userID uint) (model.User, error) {
	return s.repo.Get(ctx, userID)
}
func (s *Service) UploadEnabled() bool { return s.store != nil }

func (s *Service) Upload(ctx context.Context, userID uint, input io.Reader) (model.User, error) {
	if _, err := s.repo.Get(ctx, userID); err != nil {
		return model.User{}, err
	}
	data, err := normalizeImage(input)
	if err != nil {
		return model.User{}, err
	}
	if s.store == nil {
		return model.User{}, ErrStorageUnavailable
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return model.User{}, err
	}
	key := fmt.Sprintf("avatars/%d/%s.jpg", userID, hex.EncodeToString(nonce[:]))
	url, err := s.store.Put(ctx, key, data)
	if err != nil {
		log.Printf("avatar upload failed: %v", err)
		return model.User{}, ErrStorageUnavailable
	}
	user, old, err := s.repo.SetAvatar(ctx, userID, url)
	if err != nil {
		s.cleanup(url)
		return model.User{}, err
	}
	if old != "" && old != url {
		s.cleanup(old)
	}
	return user, nil
}

func (s *Service) Remove(ctx context.Context, userID uint) (model.User, error) {
	user, old, err := s.repo.SetAvatar(ctx, userID, "")
	if err == nil && old != "" {
		s.cleanup(old)
	}
	return user, err
}

// 数据库提交后清理旧文件；请求取消不影响清理，失败不会撤销已保存的头像。
func (s *Service) cleanup(url string) {
	if s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.store.Delete(ctx, url); err != nil {
		log.Printf("avatar cleanup failed: %v", err)
	}
}

func normalizeImage(input io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(input, MaxUploadBytes+1))
	if err != nil {
		return nil, ErrInvalidImage
	}
	if len(raw) == 0 || len(raw) > MaxUploadBytes {
		return nil, ErrInvalidImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		return nil, ErrInvalidImage
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalidImage
	}
	bounds := src.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	x, y := bounds.Min.X+(bounds.Dx()-side)/2, bounds.Min.Y+(bounds.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, 512, 512))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, image.Rect(x, y, x+side, y+side), draw.Over, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
