package profile

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type cosStore struct {
	client    *cos.Client
	publicURL string
}

// 未配置时禁用上传；部分配置不允许静默启动，避免上传到错误的桶。
func NewCOSStoreFromEnv() (Store, error) {
	bucket, public := os.Getenv("NOTE_COS_BUCKET_URL"), os.Getenv("NOTE_COS_PUBLIC_URL")
	id, key := os.Getenv("NOTE_COS_SECRET_ID"), os.Getenv("NOTE_COS_SECRET_KEY")
	if bucket == "" && public == "" && id == "" && key == "" {
		return nil, nil
	}
	if bucket == "" || id == "" || key == "" {
		return nil, fmt.Errorf("COS requires NOTE_COS_BUCKET_URL, NOTE_COS_SECRET_ID and NOTE_COS_SECRET_KEY")
	}
	u, err := url.Parse(bucket)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid COS bucket URL")
	}
	if public == "" {
		public = strings.TrimRight(bucket, "/")
	}
	p, err := url.Parse(public)
	if err != nil || p.Scheme != "https" || p.Host == "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" {
		return nil, fmt.Errorf("invalid COS public URL")
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Timeout:   30 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: id, SecretKey: key},
	})
	return &cosStore{client: client, publicURL: strings.TrimRight(public, "/")}, nil
}
func (s *cosStore) Put(ctx context.Context, key string, data []byte) (string, error) {
	response, err := s.client.Object.Put(ctx, key, bytes.NewReader(data), &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "image/jpeg", CacheControl: "public, max-age=31536000, immutable"},
	})
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		return "", err
	}
	return s.publicURL + "/" + key, nil
}
func (s *cosStore) Delete(ctx context.Context, objectURL string) error {
	prefix := s.publicURL + "/avatars/"
	if !strings.HasPrefix(objectURL, prefix) {
		return nil
	}
	key := strings.TrimPrefix(objectURL, s.publicURL+"/")
	response, err := s.client.Object.Delete(ctx, key)
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	return err
}
