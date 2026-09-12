package search

// SearchQuery 是 GET 查询参数，转换成 ListQuery 后交给 Service。
type SearchQuery struct {
	Keyword  string `form:"keyword" binding:"required"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
}
