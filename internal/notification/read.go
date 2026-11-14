package notification

import (
	"context"
	"fmt"
)

// 批量已读
func (r *gormRepository) MarkRead(
	ctx context.Context,
	uid uint,
	ids []uint,
) error {
	return fmt.Errorf("batch mark read is not implemented")
}
