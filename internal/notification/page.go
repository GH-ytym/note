package notification

// Page 是通知列表的一页，包含本次读取时的同步位置和用户全部未读数。
// Next 是下一页的通知 ID 上界。
type Page struct {
	//随机数
	//代表某次revision的history唯一版本，表示缓存是哪一代
	// 同一 epoch 可以跨越多个 revision；只有重建才更换代次。
	Epoch string `json:"epoch"`

	// Revision 是本次读取时的业务版本，重建时沿用数据库版本，不一定递增。
	Revision uint64 `json:"revision,string"`


	Items []Card `json:"items"`

	// 用户全部通知的未读总数。
	UnreadCount int64 `json:"unread_count"`

	// 有下一页时，作为下一次请求的 before 参数
	// 为 0 表示本次读取时没有更多通知
	Next uint `json:"next"`
}
