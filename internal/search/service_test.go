package search

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	apperrors "note/internal/errors"
	"testing"

	"github.com/gin-gonic/gin"
)

type recordingRepository struct {
	queries []ListQuery
	err     error
}

func (r *recordingRepository) SearchTodos(_ context.Context, q ListQuery) ([]Item, int64, error) {
	r.queries = append(r.queries, q)
	return []Item{{Kind: "todo", ID: 3, GroupID: q.GroupID, Score: 100}}, 9, r.err
}

func (r *recordingRepository) SearchEvents(ctx context.Context, q ListQuery) ([]Item, int64, error) {
	return r.SearchTodos(ctx, q)
}

func (r *recordingRepository) SearchAll(ctx context.Context, q ListQuery) ([]Item, int64, error) {
	return r.SearchTodos(ctx, q)
}

func TestSearchServiceScopeAndPagination(t *testing.T) {
	repo := &recordingRepository{}
	svc := NewService(repo)
	for name, run := range map[string]func(context.Context, ListQuery) (Result, error){
		"todos": svc.SearchTodos, "events": svc.SearchEvents, "all": svc.SearchAll,
	} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				query ListQuery
				want  error
			}{
				{ListQuery{GroupID: 1, Keyword: "开会"}, apperrors.ErrGroupUnauthenticated},
				{ListQuery{UserID: 1, Keyword: "开会"}, apperrors.ErrInvalidSearchQuery},
				{ListQuery{GroupID: 1, UserID: 1, Keyword: " \t"}, apperrors.ErrInvalidSearchQuery},
				{ListQuery{GroupID: 1, UserID: 1, Keyword: "开会", Page: -1}, apperrors.ErrInvalidSearchQuery},
				{ListQuery{GroupID: 1, UserID: 1, Keyword: "开会", PageSize: 101}, apperrors.ErrInvalidSearchQuery},
				{ListQuery{GroupID: 1, UserID: 1, Keyword: "开会", Page: int(^uint(0) >> 1), PageSize: 100}, apperrors.ErrInvalidSearchQuery},
			} {
				repo.queries = nil
				if _, err := run(context.Background(), tc.query); !errors.Is(err, tc.want) {
					t.Fatalf("query %+v: got %v, want %v", tc.query, err, tc.want)
				}
				if len(repo.queries) != 0 {
					t.Fatal("invalid scope or query reached the repository")
				}
			}
			repo.queries = nil
			page, err := run(context.Background(), ListQuery{GroupID: 7, UserID: 4, Keyword: " 开会 "})
			if err != nil || len(repo.queries) != 1 {
				t.Fatalf("search: page=%+v err=%v queries=%+v", page, err, repo.queries)
			}
			want := ListQuery{GroupID: 7, UserID: 4, Keyword: "开会", Page: 1, PageSize: 20}
			if repo.queries[0] != want || page.Total != 9 || page.Page != 1 || page.PageSize != 20 || len(page.Items) != 1 || page.Items[0].Score != 100 {
				t.Fatalf("lost scope, defaults or repository pagination: query=%+v page=%+v", repo.queries[0], page)
			}
			want.Page, want.PageSize = 2, 1
			page, err = run(context.Background(), want)
			if err != nil || page.Page != 2 || page.PageSize != 1 || page.Total != 9 || len(page.Items) != 1 || repo.queries[len(repo.queries)-1] != want {
				t.Fatalf("service paginated repository results twice: page=%+v err=%v", page, err)
			}
			repo.err = apperrors.ErrGroupAccessDenied
			if _, err := run(context.Background(), want); !errors.Is(err, repo.err) {
				t.Fatalf("membership failure was not propagated: %v", err)
			}
			repo.err = nil
		})
	}
}

func TestSearchRepositoryRequiresScope(t *testing.T) {
	// 缺少身份或群组时不得发起任何数据库查询。
	repo := NewGORMRepository(nil)
	for _, run := range []func(context.Context, ListQuery) ([]Item, int64, error){repo.SearchTodos, repo.SearchEvents, repo.SearchAll} {
		for _, tc := range []struct {
			query ListQuery
			want  error
		}{
			{ListQuery{GroupID: 1, Keyword: "开会", Page: 1, PageSize: 20}, apperrors.ErrGroupUnauthenticated},
			{ListQuery{UserID: 1, Keyword: "开会", Page: 1, PageSize: 20}, apperrors.ErrInvalidSearchQuery},
		} {
			items, total, err := run(context.Background(), tc.query)
			if !errors.Is(err, tc.want) || items != nil || total != 0 {
				t.Fatalf("missing scope: items=%+v total=%d err=%v", items, total, err)
			}
		}
	}
}

func TestSearchHandlerRequiresLoginWithoutMiddleware(t *testing.T) {
	repo := &recordingRepository{}
	h := NewSearchHandler(NewService(repo))
	r := gin.New()
	r.GET("/groups/:groupID/search/todos", h.SearchTodos)
	r.GET("/groups/:groupID/search/events", h.SearchEvents)
	r.GET("/groups/:groupID/search/all", h.SearchAll)
	for _, kind := range []string{"todos", "events", "all"} {
		request := httptest.NewRequest(http.MethodGet, "/groups/1/search/"+kind+"?keyword=meeting&user_id=1", nil)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || len(repo.queries) != 0 {
			t.Fatalf("handler trusted request identity: %d %s", response.Code, response.Body.String())
		}
	}
}
