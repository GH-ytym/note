package calendarsync

import (
	"encoding/json"
	"fmt"
	"note/internal/model"
	"time"

	"gorm.io/gorm"
)

// 某个群组的日历变化
// 一次只能改一条，所以用不着数组了
type Change struct {
	GroupID uint   `json:"group_id"`
	Kind    string `json:"kind"` //todo或者event
	ID      uint   `json:"id"`
	Action  string `json:"action"` //upsert或者remove
}

// 为所有接收者（通常是全体群成员）分别保存一条日历变化任务
func EnqueueCalendar(
	tx *gorm.DB,
	uids []uint,
	change Change,
) error {
	if len(uids) == 0 {
		return nil
	}
	data, err := json.Marshal(&change)
	if err != nil {
		return fmt.Errorf("EnqueueCalendar json.Marshal: %w", err)
	}
	now := time.Now().UTC()
	//outbox里面的receiverID只是uint，所以每个人都要投一次
	// 日历不使用通知版本，Revision 保持默认值 0。
	tasks := make([]model.Outbox, 0, len(uids))
	seen := make(map[uint]struct{}, len(uids))
	for _, uid := range uids {
		//公式化去重
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		tasks = append(tasks, model.Outbox{
			ReceiverID: uid,
			Name:       "cldr",
			Data:       string(data),
			CreatedAt:  now,
		})
	}
	//批量写入
	if err := tx.CreateInBatches(&tasks, 200).Error; err != nil {
		return fmt.Errorf("EnqueueCalendar tx.CreateInBatches: %w", err)
	}
	return nil
}

// EnqueueGroup 在业务事务内向当前群成员保存同一条日历变化。
func EnqueueGroup(tx *gorm.DB, change Change) error {
	var uids []uint
	if err := tx.Model(&model.GroupMember{}).
		Where("group_id = ?", change.GroupID).
		Pluck("user_id", &uids).Error; err != nil {
		return fmt.Errorf("list calendar recipients: %w", err)
	}
	return EnqueueCalendar(tx, uids, change)
}
