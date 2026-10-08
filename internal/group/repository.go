package group

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/notification"

	"gorm.io/gorm"
)

type Repository interface {
	ByCode(ctx context.Context, code string) (model.Group, error)
	Create(ctx context.Context, item *model.Group) error
	ListByUserID(
		ctx context.Context,
		userID uint,
	) ([]model.Group, error)
	Join(ctx context.Context, userID uint, code string) (*model.GroupJoinRequest, *model.Notification, error)
	Quit(ctx context.Context, groupID uint, userID uint, target *uint) error
	ListMembers(ctx context.Context, groupID, userID uint) ([]model.GroupMember, error)
	Dismiss(ctx context.Context, groupID, userID uint) error
}

type gormRepository struct {
	db *gorm.DB
}

func (r *gormRepository) ListMembers(ctx context.Context, groupID, userID uint) ([]model.GroupMember, error) {
	members := make([]model.GroupMember, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.Group
		err := tx.Select("id").First(&item, groupID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find group before listing members: %w", err)
		}

		// 成员列表也受当前群成员边界限制。
		var count int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}
		if count == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		// Preload 使用模型的 User 字段；保留 id 供 GORM 匹配关联。
		if err := tx.Where("group_id = ?", groupID).
			Preload("User", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "username", "suffix", "nickname", "avatar")
			}).
			Order("joined_at ASC").Order("user_id ASC").
			Find(&members).Error; err != nil {
			return fmt.Errorf("list group members: %w", err)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *gormRepository) Dismiss(ctx context.Context, groupID, userID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.Group
		err := tx.Select("id", "owner_id").First(&item, groupID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find group before dismissing: %w", err)
		}

		var count int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check dismissing membership: %w", err)
		}
		if count == 0 {
			return apperrors.ErrGroupAccessDenied
		}
		if item.OwnerID != userID {
			return apperrors.ErrGroupDismissDenied
		}

		// 先删本群 Todo 的子记录，再删 Todo，最后删群。
		// todos.group_id 使用 RESTRICT，不能直接删除仍有 Todo 的群。
		todoIDs := tx.Model(&model.Todo{}).Select("id").Where("group_id = ?", groupID)
		if err := tx.Where("todo_id IN (?)", todoIDs).
			Delete(&model.TodoCompletion{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group completions: %w", err)
		}
		if err := tx.Where("todo_id IN (?)", todoIDs).
			Delete(&model.TodoDate{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group todo dates: %w", err)
		}
		if err := tx.Where("todo_id IN (?)", todoIDs).
			Delete(&model.TodoMember{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group todo roles: %w", err)
		}
		if err := tx.Where("group_id = ?", groupID).
			Delete(&model.Todo{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group todos: %w", err)
		}
		// Event 也属于群组，先清理子记录，再删除日程。
		eventIDs := tx.Model(&model.Event{}).Select("id").Where("group_id = ?", groupID)
		if err := tx.Where("event_id IN (?)", eventIDs).Delete(&model.EventDate{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group event dates: %w", err)
		}
		if err := tx.Where("event_id IN (?)", eventIDs).Delete(&model.EventMember{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group event roles: %w", err)
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&model.Event{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group events: %w", err)
		}
		if err := tx.Where("group_id = ?", groupID).
			Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete dismissed group memberships: %w", err)
		}
		if err := tx.Delete(&model.Group{}, groupID).Error; err != nil {
			return fmt.Errorf("delete dismissed group: %w", err)
		}
		return nil
	})
}

// JoinGroup
// → 查询群组最新 policy
// → 确认用户存在、是否已经入群
// → restricted / personal：拒绝
// → public / approval：按固定群号加入或申请
//
//	→ public：写入成员和权限，保存群主提醒和投递任务
//	→ approval：保存申请，保存群主审核通知和投递任务
func (r *gormRepository) Join(
	ctx context.Context,
	userID uint,
	code string,
) (
	*model.GroupJoinRequest,
	*model.Notification,
	error,
) {
	//nil就是已经入群，非nil表示需要审核
	var pending *model.GroupJoinRequest
	var notice *model.Notification
	//事务内闭包修改这两个东西
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var g model.Group
		//按固定群号查找群组及最新入群规则
		err := tx.Select("id", "name", "code", "policy", "owner_id").
			Where("code = ? AND policy <> ?", code, model.Personal).First(&g).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find joining group: %w", err)
		}

		groupID := g.ID

		//拿到当前用户资料
		var user model.User
		err = tx.Select("id", "username", "suffix", "nickname", "avatar").
			First(&user, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupUnauthenticated
		}
		if err != nil {
			return fmt.Errorf("find joining user: %w", err)
		}

		//看在不在群里
		var cnt int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id= ? AND user_id= ?", groupID, userID).
			Count(&cnt).Error; err != nil {
			return fmt.Errorf("check membership: %w", err)
		}
		//在群里就直接返回，这个时候pending依旧是nil
		if cnt > 0 {
			return nil
		}

		//不在群里的话，就继续
		//看权限
		switch g.Policy {
		//如果是restricted和personal，不允许加入（只是不允许发送申请，restricted情况下群主依旧可以邀请）
		case model.Restricted, model.Personal:
			return apperrors.ErrGroupJoinForbidden

		case model.Public, model.Approval: //允许继续

		default: //权限字符串不对
			return fmt.Errorf("invalid group policy: %q", g.Policy)
		}

		if g.Policy == model.Public {
			//public不需要审核，直接加入
			if err := r.addMember(tx, groupID, userID); err != nil {
				return err
			}

			//给群主保存一条通知
			msg := model.Notification{
				ReceiverID: g.OwnerID, //发给群主的
				ActorID:    userID,    //当前用户发过来的
				GroupID:    groupID,
				Type:       model.Joined,
				CreatedAt:  time.Now().UTC(),
			}

			if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").
				Create(&msg).Error; err != nil {
				return fmt.Errorf("create group joined notification: %w", err)
			}
			msg.Actor = &user
			msg.Group = &g
			//把这个msg单独丢进outbox表，后续投递是从outbox里面查，不是notification
			if err := enqueueNotice(tx, msg); err != nil {
				return err
			}
			//保存一份快照
			notice = &msg

			return nil
		}

		//approval
		//以当前时间点开始计时
		now := time.Now().UTC()

		//看看7天内有没有申请过而且还在审核的
		var pre model.GroupJoinRequest
		err = tx.Where(
			"group_id = ? AND kind = ? AND sender_id = ? AND status = ?",
			groupID, model.Application, userID, model.Pending,
		).Order("id DESC").First(&pre).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find pending application: %w", err)
		}

		//有就复用
		if err == nil && time.Now().UTC().Before(pre.CreatedAt.Add(7*24*time.Hour)) {
			pending = &pre
			return nil
		}

		//没有或者过期就开个新的
		req := model.GroupJoinRequest{
			GroupID:    groupID,
			Kind:       model.Application,
			SenderID:   userID,
			ReceiverID: g.OwnerID, //对群主发送
			Status:     model.Pending,
			CreatedAt:  now,
		}
		if err := tx.Omit("Group", "Sender", "Receiver").
			Create(&req).Error; err != nil {
			return fmt.Errorf("create group application: %w", err)
		}

		msg := model.Notification{
			ReceiverID:    g.OwnerID,
			ActorID:       userID,
			GroupID:       groupID,
			JoinRequestID: &req.ID,
			Type:          model.JoinRequested, //这里需要审核
			CreatedAt:     now,
		}
		if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").
			Create(&msg).Error; err != nil {
			return fmt.Errorf("create group application notification: %w", err)
		}

		msg.Actor = &user
		msg.Group = &g
		msg.JoinRequest = &req
		if err := enqueueNotice(tx, msg); err != nil {
			return err
		}
		notice = &msg
		//记录pending
		pending = &req
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return pending, notice, nil
}

// enqueueNotice 使用 Join 的同一个事务保存投递任务。
// 保存这次 Card 的 JSON 快照，后续改名、处理申请不会改写这次事件。
func enqueueNotice(tx *gorm.DB, msg model.Notification) error {
	card, err := notification.NewCard(msg)
	if err != nil {
		return fmt.Errorf("build notification outbox card: %w", err)
	}
	data, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("encode notification outbox card: %w", err)
	}

	task := model.Outbox{
		ReceiverID: msg.ReceiverID,
		Name:       "notification.created",
		Data:       string(data),
		CreatedAt:  msg.CreatedAt,
		// PublishedAt 保持 nil，等后台成功交给 Redis 后再填写
		//因为放进表里不算发布了，真的投递出去才算
	}
	if err := tx.Create(&task).Error; err != nil {
		return fmt.Errorf("create notification outbox task: %w", err)
	}
	return nil
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(
	ctx context.Context,
	item *model.Group,
) error {
	// WithContext 让数据库操作受本次请求的取消/超时控制；它本身不开启事务。
	return r.db.WithContext(ctx).
		Transaction(
			func(tx *gorm.DB) error {
				var owner model.User
				// JWT 有效也可能对应已删除的账号，因此确认用户仍存在。
				// 这里只需要确认 ID，不必查询密码、邮箱等其他字段。
				err := tx.Select("id").First(&owner, item.OwnerID).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return apperrors.ErrGroupUnauthenticated
				}
				if err != nil {
					return fmt.Errorf("find group owner: %w", err)
				}
				// 先插入群，再填入成员的 GroupID
				if err := tx.
					Omit("Owner", "Members.User"). //跳过owner和members.User
					Create(item).
					Error; err != nil {
					if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed: groups.code") {
						// GORM 翻译后不包含具体唯一索引，确认冲突的是另一个群的 Code。
						var count int64
						if check := tx.Model(&model.Group{}).Where("code = ? AND id <> ?", item.Code, item.ID).Count(&count).Error; check != nil {
							return check
						}
						if count > 0 {
							return apperrors.ErrGroupCodeConflict
						}
					}
					return fmt.Errorf("create group with owner membership: %w", err)
				}
				return nil
			},
		)
}

func (r *gormRepository) ListByUserID(
	ctx context.Context,
	userID uint,
) ([]model.Group, error) {
	items := make([]model.Group, 0)

	err := r.db.WithContext(ctx).
		Model(&model.Group{}).
		Select("groups.*").
		Joins("JOIN group_members ON group_members.group_id = groups.id").
		Where("group_members.user_id = ?", userID).
		Order("groups.created_at DESC").
		Order("groups.id DESC").
		Find(&items).
		Error

	if err != nil {
		return nil, fmt.Errorf("list groups by user: %w", err)
	}

	return items, nil
}

// 查询群组和当前群主
// ↓
// 确认退出者仍是群成员
// ↓
// 核对当前身份与 target 是否匹配
// ↓
// 如果是群主：校验接任者，更新 OwnerID
// ↓
// 删除退出者在本群的 TodoMember 和 EventMember
// ↓
// 删除退出者的 GroupMember
// ↓
// 提交事务
func (r *gormRepository) Quit(ctx context.Context, groupID uint, userID uint, target *uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group model.Group
		err := tx.Select("id", "owner_id").First(&group, groupID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrGroupNotFound
		}
		if err != nil {
			return fmt.Errorf("find group before quitting: %w", err)
		}

		var count int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check departing membership: %w", err)
		}
		if count == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		isOwner := (group.OwnerID == userID)
		//群主退群却没指定下一任群主，以及成员退群却指定下一任群主，都是不行的
		if isOwner && target == nil {
			return apperrors.ErrGroupQuitConflict
		}
		if !isOwner && target != nil {
			return apperrors.ErrGroupQuitConflict
		}

		//群主退群，先转让群主给target
		if isOwner {
			//转让给自己没意义
			if *target == 0 || *target == userID {
				return apperrors.ErrGroupTransferTargetInvalid
			}

			//转让给滚木也不行
			var targetCount int64
			if err := tx.Model(&model.GroupMember{}).
				Where("group_id = ? AND user_id = ?", groupID, *target).
				Count(&targetCount).Error; err != nil {
				return fmt.Errorf("check successor membership: %w", err)
			}
			if targetCount == 0 {
				return apperrors.ErrGroupTransferTargetInvalid
			}

			//更新
			if err := tx.Model(&model.Group{}).
				Where("id = ?", groupID).
				Update("owner_id", *target).Error; err != nil {
				return fmt.Errorf("transfer group ownership: %w", err)
			}
		}

		// 删除退出者在本群的 Todo 和 Event 权限，保留内容。
		//不会删除ta对todo的完成记录
		groupTodoIDs := tx.Model(&model.Todo{}).
			Select("id").
			Where("group_id = ?", groupID)
		if err := tx.
			Where(
				"user_id = ? AND todo_id IN (?)",
				userID,
				groupTodoIDs,
			).
			Delete(&model.TodoMember{}).Error; err != nil {
			return fmt.Errorf("delete departing todo roles: %w", err)
		}
		groupEventIDs := tx.Model(&model.Event{}).Select("id").Where("group_id = ?", groupID)
		if err := tx.Where("user_id = ? AND event_id IN (?)", userID, groupEventIDs).
			Delete(&model.EventMember{}).Error; err != nil {
			return fmt.Errorf("delete departing event roles: %w", err)
		}

		// 删除退出者的group-user关系
		if err := tx.
			Where("group_id = ? AND user_id = ?", groupID, userID).
			Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete departing membership: %w", err)
		}

		return nil
	})
}

// 写入成员、初始化权限的函数
// 使用调用方的事务
func (r *gormRepository) addMember(
	tx *gorm.DB,
	groupID uint,
	userID uint,
) error {
	member := model.GroupMember{
		GroupID: groupID,
		UserID:  userID,
	}
	if err := tx.Omit("User").Create(&member).Error; err != nil {
		return fmt.Errorf("create group membership: %w", err)
	}

	var todos []model.Todo
	if err := tx.Select("id", "creator_id").
		Where("group_id = ?", groupID).
		Find(&todos).Error; err != nil {
		return fmt.Errorf("find group todos: %w", err)
	}

	roles := make([]model.TodoMember, 0, len(todos))
	//给每个todo增加一条权限记录
	for _, todo := range todos {
		role := model.TodoViewer
		//这里只可能是user之前退群过，但todo记录还在
		//所以加群后这条todo对他而言依旧是creator，要不然没人能动它了
		if todo.CreatorID == userID {
			role = model.TodoEditor
		}

		roles = append(roles, model.TodoMember{
			TodoID: todo.ID,
			UserID: userID,
			Role:   role,
		})
	}

	//批量写入免得频繁动sql
	if len(roles) > 0 {
		if err := tx.Omit("User").
			CreateInBatches(&roles, 200).Error; err != nil {
			return fmt.Errorf("initialize todo roles: %w", err)
		}
	}

	var events []model.Event
	if err := tx.Select("id", "creator_id").
		Where("group_id = ?", groupID).
		Find(&events).Error; err != nil {
		return fmt.Errorf("find group events: %w", err)
	}

	//给每个event增加一条权限记录
	roles1 := make([]model.EventMember, 0, len(events))
	for _, event := range events {
		role := model.EventViewer
		//跟todo一个道理
		if event.CreatorID == userID {
			role = model.EventEditor
		}

		roles1 = append(roles1, model.EventMember{
			EventID: event.ID,
			UserID:  userID,
			Role:    role,
		})
	}

	if len(roles1) > 0 {
		if err := tx.Omit("User").
			CreateInBatches(&roles1, 200).Error; err != nil {
			return fmt.Errorf("initialize event roles: %w", err)
		}
	}

	return nil
}

// 只返回可公开查找群组的预览资料，不加载成员和群内内容。
func (r *gormRepository) ByCode(ctx context.Context, code string) (model.Group, error) {
	var item model.Group
	err := r.db.WithContext(ctx).Select("id", "name", "code", "policy").Where("code = ? AND policy <> ?", code, model.Personal).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, apperrors.ErrGroupNotFound
	}
	return item, err
}
