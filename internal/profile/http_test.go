package profile_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"note/internal/auth"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/profile"
	"note/internal/router"
	"testing"
	"time"
)

func TestProfileRoutesUseJWTIdentity(t *testing.T) {
	db, actor, _, service := fixture(t)
	other := model.User{Username: "bob", Suffix: 12345, Nickname: "Bob", Email: "bob@example.com", PasswordHash: "private-hash"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	manager, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Generate(actor.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := router.NewWithWeb(nil, nil, nil, nil, nil, nil, nil, manager, "", handler.NewProfileHandler(service))
	for _, prefix := range []string{"", "/api"} {
		for _, endpoint := range []struct{ method, path string }{{"GET", "/users/me"}, {"PUT", "/users/me/avatar"}, {"DELETE", "/users/me/avatar"}} {
			response := httptest.NewRecorder()
			r.ServeHTTP(response, httptest.NewRequest(endpoint.method, prefix+endpoint.path, nil))
			if response.Code != 401 {
				t.Fatalf("unprotected %s: %d", endpoint.path, response.Code)
			}
		}
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("user_id", "2"); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("avatar", "picture.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(picture(20, 20)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/users/me/avatar", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatalf("upload: %d %s", response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != float64(actor.ID) || result["avatar"] == "" || result["avatar_upload_enabled"] != true {
		t.Fatalf("wrong identity: %+v", result)
	}
	for _, field := range []string{"email", "Email", "password_hash", "PasswordHash"} {
		if _, ok := result[field]; ok {
			t.Fatalf("leaked %s", field)
		}
	}
	if err := db.First(&other, other.ID).Error; err != nil || other.Avatar != "" {
		t.Fatal("client user_id changed someone else's avatar")
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		path := "/api/users/me"
		if method == http.MethodDelete {
			path += "/avatar"
		}
		req = httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response = httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != 200 {
			t.Fatalf("%s: %d", method, response.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPut, "/api/users/me/avatar", bytes.NewBufferString(`{"avatar":"https://other.example/image.jpg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != 400 {
		t.Fatalf("arbitrary URL accepted: %d", response.Code)
	}
}

func TestCOSConfiguration(t *testing.T) {
	for _, key := range []string{"NOTE_COS_BUCKET_URL", "NOTE_COS_PUBLIC_URL", "NOTE_COS_SECRET_ID", "NOTE_COS_SECRET_KEY"} {
		t.Setenv(key, "")
	}
	store, err := profile.NewCOSStoreFromEnv()
	if store != nil || err != nil {
		t.Fatal("empty config should disable uploads")
	}
	t.Setenv("NOTE_COS_SECRET_ID", "test-id")
	if _, err := profile.NewCOSStoreFromEnv(); err == nil {
		t.Fatal("partial config accepted")
	}
	t.Setenv("NOTE_COS_SECRET_KEY", "test-key")
	t.Setenv("NOTE_COS_BUCKET_URL", "https://example.cos.ap-shanghai.myqcloud.com")
	if store, err := profile.NewCOSStoreFromEnv(); err != nil || store == nil {
		t.Fatalf("valid config: %v", err)
	}
	t.Setenv("NOTE_COS_PUBLIC_URL", "http://example.com")
	if _, err := profile.NewCOSStoreFromEnv(); err == nil {
		t.Fatal("insecure public URL accepted")
	}
}
