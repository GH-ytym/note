package search

import (
	"context"
	apperrors "note/internal/errors"
	"strings"
)

type SearchService interface {
	SearchTodos(context.Context, ListQuery) (Result, error)
	SearchEvents(context.Context, ListQuery) (Result, error)
	SearchAll(context.Context, ListQuery) (Result, error)
}
type service struct{ repo SearchRepository }

func NewService(repo SearchRepository) SearchService { return &service{repo: repo} }
func (s *service) SearchTodos(ctx context.Context, q ListQuery) (Result, error) {
	return searchPage(ctx, q, s.repo.SearchTodos)
}
func (s *service) SearchEvents(ctx context.Context, q ListQuery) (Result, error) {
	return searchPage(ctx, q, s.repo.SearchEvents)
}
func (s *service) SearchAll(ctx context.Context, q ListQuery) (Result, error) {
	return searchPage(ctx, q, s.repo.SearchAll)
}

// Preserve the repository's score and page; never paginate a second time.
func searchPage(ctx context.Context, q ListQuery, run func(context.Context, string, int, int) ([]Item, int64, error)) (Result, error) {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	if q.Keyword == "" || q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || q.Page-1 > int(^uint(0)>>1)/q.PageSize {
		return Result{}, apperrors.ErrInvalidSearchQuery
	}
	items, total, err := run(ctx, q.Keyword, q.Page, q.PageSize)
	if err != nil {
		return Result{}, err
	}
	return Result{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
