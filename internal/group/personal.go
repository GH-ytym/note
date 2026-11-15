package group

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"note/internal/model"
	"note/internal/utils"
)

// EnsurePersonal 在调用者的事务中创建个人群组及成员关系，可重复调用。
func EnsurePersonal(tx *gorm.DB, userID uint) error {
	var item model.Group
	if err := tx.Where("owner_id = ? AND policy = ?", userID, model.Personal).Find(&item).Error; err != nil {
		return err
	}
	if item.ID == 0 {
		for i := 0; i < 10; i++ {
			code, err := utils.GenerateGroupCode()
			if err != nil {
				return err
			}
			item = model.Group{Name: "个人空间", OwnerID: userID, Policy: model.Personal, Code: code}
			result := tx.Omit("Owner", "Members").Clauses(clause.OnConflict{DoNothing: true}).Create(&item)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				break
			}
			item.ID = 0
		}
		if item.ID == 0 {
			return fmt.Errorf("create personal group: no available group code")
		}
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.GroupMember{GroupID: item.ID, UserID: userID}).Error
}
