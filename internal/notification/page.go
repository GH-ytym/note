package notification

// 通知列表的一页
type Page struct {
	Epoch string `json:"epoch"`

	Revision uint64 `json:"revision,string"`

	Items []Card `json:"items"`

	// 用户全部通知的未读总数。
	UnreadCount int64 `json:"unread_count"`

	// 有下一页时，作为下一次请求的 before 参数
	// 为 0 表示本次读取时没有更多通知
	Next uint `json:"next"`
}
