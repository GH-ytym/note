package profile_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"note/internal/model"
	"note/internal/profile"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

type memoryStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	putErr    error
	deleteErr error
}

func (s *memoryStore) Put(_ context.Context, key string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return "", s.putErr
	}
	url := "https://avatars.example/" + key
	s.objects[url] = append([]byte(nil), data...)
	return url, nil
}
func (s *memoryStore) Delete(_ context.Context, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, url)
	return nil
}
func fixture(t *testing.T) (*gorm.DB, model.User, *memoryStore, *profile.Service) {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "profile.db"))
	db, err := gorm.Open(gormlite.Open("file:"+path+"?_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "alice", Suffix: 12345, Nickname: "Alice", Email: "private@example.com", PasswordHash: "private-hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{objects: make(map[string][]byte)}
	return db, user, store, profile.NewService(profile.NewRepository(db), store)
}
func picture(width, height int) []byte {
	im := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: 180, G: 100, B: 30, A: 128})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, im); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

func TestAvatarUploadReplaceAndRemove(t *testing.T) {
	_, actor, store, service := fixture(t)
	ctx := context.Background()
	first, err := service.Upload(ctx, actor.ID, bytes.NewReader(picture(40, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Avatar, "/avatars/1/") {
		t.Fatalf("unexpected URL: %s", first.Avatar)
	}
	decoded, format, err := image.Decode(bytes.NewReader(store.objects[first.Avatar]))
	if err != nil || format != "jpeg" || decoded.Bounds().Dx() != 512 || decoded.Bounds().Dy() != 512 {
		t.Fatalf("invalid processed image: %s %v", format, err)
	}
	second, err := service.Upload(ctx, actor.ID, bytes.NewReader(picture(10, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if first.Avatar == second.Avatar || len(store.objects) != 1 {
		t.Fatal("replacement did not remove old object or reused URL")
	}
	stored, err := service.Get(ctx, actor.ID)
	if err != nil || stored.Avatar != second.Avatar {
		t.Fatalf("database avatar: %+v, %v", stored, err)
	}
	removed, err := service.Remove(ctx, actor.ID)
	if err != nil || removed.Avatar != "" || len(store.objects) != 0 {
		t.Fatalf("remove: %+v, %v", removed, err)
	}
}

func TestInvalidAvatarsNeverUpload(t *testing.T) {
	_, actor, store, service := fixture(t)
	for name, data := range map[string][]byte{
		"empty": nil, "not an image": []byte("hello"), "SVG": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
		"too many bytes": bytes.Repeat([]byte{'a'}, profile.MaxUploadBytes+1), "too wide": picture(4097, 1),
		"truncated PNG": picture(20, 20)[:40],
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Upload(context.Background(), actor.ID, bytes.NewReader(data))
			if !errors.Is(err, profile.ErrInvalidImage) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if len(store.objects) != 0 {
		t.Fatal("invalid image reached cloud storage")
	}
}

func TestAvatarFailuresPreserveDatabaseAndCleanUp(t *testing.T) {
	db, actor, store, service := fixture(t)
	ctx := context.Background()
	first, err := service.Upload(ctx, actor.ID, bytes.NewReader(picture(20, 20)))
	if err != nil {
		t.Fatal(err)
	}
	store.putErr = errors.New("cloud offline")
	if _, err := service.Upload(ctx, actor.ID, bytes.NewReader(picture(20, 20))); !errors.Is(err, profile.ErrStorageUnavailable) {
		t.Fatal(err)
	}
	store.putErr = nil
	if err := db.Exec(`CREATE TRIGGER fail_avatar BEFORE UPDATE OF avatar ON users BEGIN SELECT RAISE(ABORT, 'test update failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Upload(ctx, actor.ID, bytes.NewReader(picture(20, 20))); err == nil {
		t.Fatal("expected database failure")
	}
	stored, err := service.Get(ctx, actor.ID)
	if err != nil || stored.Avatar != first.Avatar || len(store.objects) != 1 {
		t.Fatal("failed upload changed avatar or leaked new object")
	}
	if err := db.Exec(`DROP TRIGGER fail_avatar`).Error; err != nil {
		t.Fatal(err)
	}
	store.deleteErr = errors.New("cleanup offline")
	if user, err := service.Remove(ctx, actor.ID); err != nil || user.Avatar != "" {
		t.Fatalf("cleanup failure rolled back committed removal: %v", err)
	}
}

func TestConcurrentAvatarsRetainOnlyTheCommittedObject(t *testing.T) {
	_, actor, store, service := fixture(t)
	data := picture(20, 20)
	errorsCh := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			_, err := service.Upload(context.Background(), actor.ID, bytes.NewReader(data))
			errorsCh <- err
		})
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := service.Get(context.Background(), actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.objects) != 1 || store.objects[stored.Avatar] == nil {
		t.Fatalf("current object was cleaned incorrectly: %s, %d remaining", stored.Avatar, len(store.objects))
	}
}

func TestMissingStorageAndUser(t *testing.T) {
	db, actor, store, service := fixture(t)
	_, err := service.Upload(context.Background(), actor.ID+100, bytes.NewReader(picture(10, 10)))
	if !errors.Is(err, gorm.ErrRecordNotFound) || len(store.objects) != 0 {
		t.Fatalf("unknown user: %v", err)
	}
	service = profile.NewService(profile.NewRepository(db), nil)
	if service.UploadEnabled() {
		t.Fatal("unconfigured storage enabled")
	}
	_, err = service.Upload(context.Background(), actor.ID, bytes.NewReader(picture(10, 10)))
	if !errors.Is(err, profile.ErrStorageUnavailable) {
		t.Fatal(err)
	}
}
