package auth

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
	apperrors "note/internal/errors"
	"note/internal/model"
)

// 嵌入接口，仅实现本测试会调用的方法。
type registrationRepositoryStub struct {
	Repository
	calls       int
	emailChecks int
	failures    int
	failure     error
	suffixes    []int
}

func (r *registrationRepositoryStub) FindByEmail(context.Context, string) (*model.User, error) {
	r.emailChecks++
	return nil, gorm.ErrRecordNotFound
}

func (r *registrationRepositoryStub) Create(_ context.Context, user *model.User) error {
	r.calls++
	r.suffixes = append(r.suffixes, user.Suffix)
	if r.calls <= r.failures {
		return r.failure
	}
	user.ID = 1
	return nil
}

func TestRegistrationRetryClassification(t *testing.T) {
	for _, tc := range []struct {
		name            string
		failure         error
		failures, calls int
		want            error
	}{
		{"account collision retries", apperrors.ErrAccountTaken, 1, 2, nil},
		{"email collision stops", apperrors.ErrEmailTaken, 1, 1, apperrors.ErrEmailTaken},
		{"last candidate succeeds", apperrors.ErrAccountTaken, 14, 15, nil},
		{"retry limit", apperrors.ErrAccountTaken, 15, 15, apperrors.ErrSuffixUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &registrationRepositoryStub{failure: tc.failure, failures: tc.failures}
			_, err := NewService(repo).Register(context.Background(), "小明", "ming@example.com", "Demo_12345")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if repo.calls != tc.calls {
				t.Fatalf("calls = %d, want %d", repo.calls, tc.calls)
			}
			if repo.emailChecks != 1 {
				t.Fatal("service must not recheck email inside suffix retry loop")
			}
			seen := map[int]bool{}
			for _, suffix := range repo.suffixes {
				if suffix < 10000 || suffix > 99999 || seen[suffix] {
					t.Fatalf("invalid or repeated suffix: %d", suffix)
				}
				seen[suffix] = true
			}
		})
	}
}

func TestGenerateSuffixes(t *testing.T) {
	suffixes, err := generateSuffixes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, suffix := range suffixes {
		if suffix < 10000 || suffix > 99999 || seen[suffix] {
			t.Fatalf("invalid or duplicate suffix: %d", suffix)
		}
		seen[suffix] = true
	}
	if len(seen) != 15 {
		t.Fatal("expected 15 unique suffixes")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := generateSuffixes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
