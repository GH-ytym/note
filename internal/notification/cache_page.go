package notification

import (
	"fmt"
	"strconv"
)

// 生成分页查询的参数
func pgArgs(
	uid uint, //用户
	before uint, //取id小于xxx的全部通知  0就是从头开始
	limit uint, //每页多少条
) (
	[]string, //max
	[]any, //多少条+1，多一条来检查有没有下一页
	error, //错误
) {
	if uid == 0 {
		return nil, nil, fmt.Errorf("invalid uid")
	}
	if uint64(before) > uint64(1<<53-1) {
		return nil, nil, fmt.Errorf("invalid notification cursor")
	}
	if limit < 1 || limit > 100 {
		return nil, nil, fmt.Errorf("page limit must be between 1 and 100")
	}

	k := notificationCacheKeys(uid)

	//这三个地方存了通知
	keys := []string{
		k.state,
		k.items,
		k.order,
	}

	//before如果是0，就是第一页，此时从最大id开始
	max := "+inf"

	// 后续页排除上一页最后一条通知
	//(111 是redis的特殊写法，表示小于111的数字
	if before != 0 {
		max = "(" + strconv.FormatUint(uint64(before), 10)
	}

	args := []any{
		max,
		limit + 1, //多拿一条
	}

	return keys, args, nil
}
