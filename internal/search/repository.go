package search

import (
	"context"
	"database/sql"
	"fmt"
	apperrors "note/internal/errors"
	"note/internal/model"
	"strings"

	"gorm.io/gorm"
)

// The repository owns scoring, stable ordering and pagination for every search.
type Repository interface {
	SearchTodos(context.Context, ListQuery) ([]Item, int64, error)
	SearchEvents(context.Context, ListQuery) ([]Item, int64, error)
	SearchAll(context.Context, ListQuery) ([]Item, int64, error)
}
type gormRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

const todoMatchesSQL = `SELECT 'todo' AS kind, id, group_id, title, content, color, starts_at,
 NULL AS ends_at, updated_at FROM todos
 WHERE group_id = @group_id
 AND (title LIKE @contains ESCAPE '!' OR content LIKE @contains ESCAPE '!')`
const eventMatchesSQL = `SELECT 'event' AS kind, id, group_id, title, content, color, starts_at,
 ends_at, updated_at FROM events
 WHERE group_id = @group_id
 AND (title LIKE @contains ESCAPE '!' OR content LIKE @contains ESCAPE '!')`

func (r *gormRepository) SearchTodos(ctx context.Context, q ListQuery) ([]Item, int64, error) {
	return r.searchMatches(ctx, todoMatchesSQL, q)
}
func (r *gormRepository) SearchEvents(ctx context.Context, q ListQuery) ([]Item, int64, error) {
	return r.searchMatches(ctx, eventMatchesSQL, q)
}
func (r *gormRepository) SearchAll(ctx context.Context, q ListQuery) ([]Item, int64, error) {
	return r.searchMatches(ctx, todoMatchesSQL+" UNION ALL "+eventMatchesSQL, q)
}

func (r *gormRepository) searchMatches(
	ctx context.Context,
	matchedSQL string,
	q ListQuery,
) ([]Item, int64, error) {
	if q.UserID == 0 {
		return nil, 0, apperrors.ErrGroupUnauthenticated
	}
	if q.GroupID == 0 {
		return nil, 0, apperrors.ErrInvalidSearchQuery
	}
	escaped := escapeKeyword(q.Keyword)

	args := map[string]any{
		"group_id": q.GroupID,
		"keyword":  q.Keyword,
		"prefix":   escaped + "%",
		"contains": "%" + escaped + "%",
		"limit":    q.PageSize,
		"offset":   (q.Page - 1) * q.PageSize,
	}

	items := make([]Item, 0)
	var total int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 成员资格、总数和分页结果使用同一份数据库快照。
		// 残留的 Todo / Event 编辑授权不能代替当前群成员资格。
		var memberCount int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", q.GroupID, q.UserID).
			Count(&memberCount).Error; err != nil {
			return fmt.Errorf("check search group membership: %w", err)
		}
		if memberCount == 0 {
			return apperrors.ErrGroupAccessDenied
		}
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
	}, &sql.TxOptions{ReadOnly: true})

	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// 三个入口都把用户输入中的通配符当作普通字符。
func escapeKeyword(keyword string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(keyword)
}
