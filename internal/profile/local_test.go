package profile_test

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"image"
	"net/http/httptest"
	"note/internal/profile"
	"note/internal/router"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalAvatarLifecycle(t *testing.T) {
	db, user, _, _ := fixture(t)
	dir := t.TempDir()
	store, err := profile.NewLocalStore(dir, "/p/note/api/media")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := profile.NewService(profile.NewRepository(db), store)
	first, err := svc.Upload(context.Background(), user.ID, bytes.NewReader(picture(30, 20)))
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	router.RegisterAvatarFiles(r, store)
	fetch := func(url string, want int) []byte {
		t.Helper()
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest("GET", strings.TrimPrefix(url, "/p/note"), nil))
		if res.Code != want {
			t.Fatalf("%s: status %d", url, res.Code)
		}
		if want == 200 && (res.Header().Get("Content-Type") != "image/jpeg" || res.Header().Get("X-Content-Type-Options") != "nosniff") {
			t.Fatal("missing image headers")
		}
		return res.Body.Bytes()
	}
	data := fetch(first.Avatar, 200)
	im, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "jpeg" || im.Bounds().Dx() != 512 {
		t.Fatalf("image: %s %v", format, err)
	}
	// Vite 的 /api 代理会剥离 /api 前缀。
	fetch(strings.Replace(first.Avatar, "/api/media/", "/media/", 1), 200)
	reopened, err := profile.NewLocalStore(dir, "/p/note/api/media")
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	next, err := svc.Upload(context.Background(), user.ID, bytes.NewReader(picture(12, 12)))
	if err != nil {
		t.Fatal(err)
	}
	fetch(first.Avatar, 404)
	fetch(next.Avatar, 200)
	if _, err := svc.Remove(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}
	fetch(next.Avatar, 404)
}

func TestLocalAvatarRejectsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.db"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := profile.NewLocalStore(dir, "/api/media")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, key := range []string{"../note.db", "note.db", "avatars/1/../../note.db", "avatars/1/1.jpg", "avatars/", "avatars/1/a.svg"} {
		if _, err := store.Put(context.Background(), key, []byte("bad")); err == nil {
			t.Fatalf("accepted %s", key)
		}
		res := httptest.NewRecorder()
		store.ServeHTTP(res, httptest.NewRequest("GET", "/"+key, nil))
		if res.Code != 404 {
			t.Fatalf("read %s: %d", key, res.Code)
		}
		if err := store.Delete(context.Background(), "/api/media/"+key); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "note.db")); err != nil {
		t.Fatal("deleted unrelated file", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Put(ctx, "avatars/1/0123456789abcdef0123456789abcdef.jpg", nil); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, base := range []string{"https://evil.test", "//evil.test", "/api/../data", "/api/media?x=1"} {
		if _, err := profile.NewLocalStore(dir, base); err == nil {
			t.Fatalf("accepted URL %s", base)
		}
	}
}
