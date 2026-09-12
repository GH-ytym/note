package search

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
)

// The repository owns scoring, stable ordering and pagination for every search.
type SearchRepository interface {
	SearchTodos(context.Context, string, int, int) ([]Item, int64, error)
	SearchEvents(context.Context, string, int, int) ([]Item, int64, error)
	SearchAll(context.Context, string, int, int) ([]Item, int64, error)
}
type gormRepository struct{ db *gorm.DB }

func NewGORMRepository(db *gorm.DB) SearchRepository { return &gormRepository{db: db} }

const todoMatchesSQL = `SELECT 'todo' AS kind, id, title, content, color, starts_at,
 NULL AS ends_at, updated_at FROM todos
 WHERE title LIKE @contains ESCAPE '!' OR content LIKE @contains ESCAPE '!'`
const eventMatchesSQL = `SELECT 'event' AS kind, id, title, content, color, starts_at,
 ends_at, updated_at FROM events
 WHERE title LIKE @contains ESCAPE '!' OR content LIKE @contains ESCAPE '!'`

func (r *gormRepository) SearchTodos(ctx context.Context, keyword string, page, pageSize int) ([]Item, int64, error) {
	return r.searchMatches(ctx, todoMatchesSQL, keyword, page, pageSize)
}
func (r *gormRepository) SearchEvents(ctx context.Context, keyword string, page, pageSize int) ([]Item, int64, error) {
	return r.searchMatches(ctx, eventMatchesSQL, keyword, page, pageSize)
}
func (r *gormRepository) SearchAll(ctx context.Context, keyword string, page, pageSize int) ([]Item, int64, error) {
	return r.searchMatches(ctx, todoMatchesSQL+" UNION ALL "+eventMatchesSQL, keyword, page, pageSize)
}

func (r *gormRepository) searchMatches(
	ctx context.Context,
	matchedSQL string,
	keyword string,
	page, pageSize int,
) ([]Item, int64, error) {
	escaped := escapeKeyword(keyword)

	args := map[string]interface{}{
		"keyword":  keyword,
		"prefix":   escaped + "%",
		"contains": "%" + escaped + "%",
		"limit":    pageSize,
		"offset":   (page - 1) * pageSize,
	}

	items := make([]Item, 0)
	var total int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//使用事务保证查询的时候是同一份数据库快照
		// 第一次查询：统计两表合并后的匹配总数，不分页。
		countSQL := `
			SELECT COUNT(*)
			FROM (` + matchedSQL + `) AS matched
		`

		if err := tx.Raw(countSQL, args).Scan(&total).Error; err != nil {
			return fmt.Errorf("count search results: %w", err)
		}

		// 第二次查询：对合并结果评分、排序，最后统一分页。
		listSQL := `
			SELECT
				matched.*,
				CASE
					WHEN title = @keyword THEN 100
					WHEN title LIKE @prefix ESCAPE '!' THEN 80
					WHEN title LIKE @contains ESCAPE '!' THEN 60
					WHEN content LIKE @contains ESCAPE '!' THEN 20
					ELSE 0
				END AS score
			FROM (` + matchedSQL + `) AS matched
			ORDER BY
				score DESC,
				updated_at DESC,
				kind ASC,
				id DESC
			LIMIT @limit OFFSET @offset
		`

		if err := tx.Raw(listSQL, args).Scan(&items).Error; err != nil {
			return fmt.Errorf("query search results: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// 三个入口都把用户输入中的通配符当作普通字符。
func escapeKeyword(keyword string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(keyword)
}
