package search

import (
	"time"
)

// ListQuery 是业务查询条件，不依赖 HTTP 参数绑定。
type ListQuery struct {
	Keyword  string
	Page     int
	PageSize int
}

// Item 是搜索结果，不是数据库表，也不展开周期实例。
// Kind 和 ID 共同标识记录；不同表的 ID 可能相同。
type Item struct {
	Kind      string     `json:"kind"`
	ID        uint       `json:"id"`
	Title     string     `json:"title"`
	Content   *string    `json:"content"`
	Color     string     `json:"color"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Score     int        `json:"score" gorm:"column:score"`
}
type Result struct {
	Items    []Item `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
