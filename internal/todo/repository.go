package todo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"note/internal/model"
	"slices"
	"time"

	apperrors "note/internal/errors"

	"github.com/teambition/rrule-go"
	"gorm.io/gorm"
)

// Repository 声明 Service 所需的数据库操作。
type Repository interface {
	Create(ctx context.Context, item *model.Todo) error
	List(ctx context.Context, query ListQuery) ([]model.Todo, int64, error)
	//在指定时间段找所有可能出现的Todo（只缩小范围不保证全对）
	CalendarCandidates(
		ctx context.Context,
		userID uint,
		from time.Time,
		to time.Time,
	) ([]model.Todo, error)
	ByID(ctx context.Context, id uint) (model.Todo, error)
	ByIDForUser(ctx context.Context, id uint, userID uint) (model.Todo, error)
	Patch(ctx context.Context, id uint, userID uint, command PatchCommand) (model.Todo, error)
	Delete(ctx context.Context, id uint, userID uint) error
	//给单个todo的某一天设置完成状态
	SetOccurrenceDone(
		ctx context.Context,
		todoID uint,
		userID uint,
		occursOn time.Time,
		done bool,
	) error
	//查询某一条todo某一天的完成名单
	GetOccurrenceCompletions(
		ctx context.Context,
		todoID uint,
		userID uint,
		occursOn time.Time,
	) ([]CompletionUser, error)
	//查询某段时间范围内的todo完成情况
	CompletionsInRange(
		ctx context.Context,
		todoIDs []uint,
		from time.Time,
		to time.Time,
	) ([]model.TodoCompletion, error)
	PatchRole(
		ctx context.Context,
		actorID, todoID uint,
		userIDs []uint,
		role model.TodoRole,
	) error

	// 一次取得日历所需的数据，保证来自同一个数据库快照
	CalendarData(
		ctx context.Context,
		userID uint,
		from time.Time,
		to time.Time,
	) ([]model.Todo, []model.TodoCompletion, error)
}

// gormRepository 是 Repository 的 GORM 实现，对 todo 包外隐藏。
type gormRepository struct {
	db *gorm.DB
}

func (r *gormRepository) PatchRole(ctx context.Context, actorID, todoID uint, userIDs []uint, role model.TodoRole) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//找todo，和它的creator与group
		var todo model.Todo
		err := tx.Select("id", "group_id", "creator_id").
			First(&todo, todoID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrTodoNotFound
		}
		if err != nil {
			return fmt.Errorf("find todo: %w", err)
		}

		//actor必须在群里
		var cnt int64
		if err := tx.Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id = ?",
				todo.GroupID,
				actorID,
			).
			Count(&cnt).Error; err != nil {
			return fmt.Errorf("check actor membership: %w", err)
		}
		if cnt == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		//actor还必须是这个todo的创建者
		if todo.CreatorID != actorID {
			return apperrors.ErrTodoPermissionDenied
		}

		//创建者不能被改为viewer
		if role == model.TodoViewer {
			if slices.Contains(userIDs, todo.CreatorID) {
				return apperrors.ErrTodoRoleInvalid
			}
		}

		//所有userIDs都要在群组内
		//查groupmember
		var cnt1 int64
		if err := tx.Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id IN ?",
				todo.GroupID,
				userIDs,
			).
			Count(&cnt1).Error; err != nil {
			return fmt.Errorf("check target memberships: %w", err)
		}
		if cnt1 != int64(len(userIDs)) {
			return apperrors.ErrTodoMemberNotFound
		}

		// 所有人都必须有这条 Todo 的授权记录
		//查todomember
		var cnt2 int64
		if err := tx.Model(&model.TodoMember{}).
			Where(
				"todo_id = ? AND user_id IN ?",
				todoID,
				userIDs,
			).
			Count(&cnt2).Error; err != nil {
			return fmt.Errorf("check todo members: %w", err)
		}
		if cnt2 != int64(len(userIDs)) {
			return apperrors.ErrTodoMemberNotFound
		}

		//一次性修改
		if err := tx.Model(&model.TodoMember{}).
			Where(
				"todo_id = ? AND user_id IN ?",
				todoID,
				userIDs,
			).
			Update("role", role).
			Error; err != nil {
			return fmt.Errorf("update todo member roles: %w", err)
		}

		return nil
	})
}

func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CalendarCandidates(
	ctx context.Context,
	userID uint,
	from time.Time,
	to time.Time,
) ([]model.Todo, error) {
	items := make([]model.Todo, 0)
	fromDate := from.Format(time.DateOnly)
	toDate := to.Format(time.DateOnly)

	err := r.db.
		WithContext(ctx).
		Where(`EXISTS (SELECT 1 FROM group_members gm
			WHERE gm.group_id = todos.group_id AND gm.user_id = ?)`, userID).
		Preload(
			//custom模式
			//把自定义todo的具体日期读取出来
			"CustomDates",
			"date >= ? AND date < ?",
			fromDate,
			toDate,
		).
		Where( //1.一次性，startsat在范围内就算
			//2.普通重复，只要startsat在to之前都有可能
			//3，custom，自定义日期在from和to之间
			`
		starts_at IS NOT NULL
		AND (
			(
				repeat_mode = ?
				AND starts_at >= ?
				AND starts_at < ?
			)
			OR
			(
				repeat_mode NOT IN (?, ?)
				AND starts_at < ?
			)
			OR
			(
				repeat_mode = ?
				AND EXISTS (
					SELECT 1
					FROM todo_dates
					WHERE todo_dates.todo_id = todos.id
						AND todo_dates.date >= ?
						AND todo_dates.date < ?
				)
			)
		)
	`,
			model.RepeatOnce,
			from,
			to,

			model.RepeatOnce,
			model.RepeatCustom,
			to,

			model.RepeatCustom,
			fromDate,
			toDate,
		).
		Order("starts_at ASC").
		Find(&items).
		Error

	if err != nil {
		return nil, fmt.Errorf("list calendar candidates: %w", err)
	}

	return items, nil
}

func (r *gormRepository) Create(ctx context.Context, item *model.Todo) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//找到目标群组全体成员
		var members []model.GroupMember
		err := tx.Select("user_id").
			Where("group_id = ?", item.GroupID).
			Find(&members).Error
		if err != nil {
			return fmt.Errorf("list group members: %w", err)
		}

		//创建者也需要是当前群里的人
		yes := false
		for _, member := range members {
			if member.UserID == item.CreatorID {
				yes = true
				break
			}
		}
		if !yes {
			return apperrors.ErrGroupAccessDenied
		}

		//保存todo和customdates
		err = tx.Omit("Group", "Creator", "Members").Create(item).Error
		if err != nil {
			return fmt.Errorf("create todo:%w", err)
		}

		//填写权限记录
		item.Members = make([]model.TodoMember, 0, len(members))
		for _, member := range members {
			role := model.TodoViewer
			//创建者是editor
			if member.UserID == item.CreatorID {
				role = model.TodoEditor
			}
			item.Members = append(item.Members, model.TodoMember{
				TodoID: item.ID,
				UserID: member.UserID,
				Role:   role,
			})
		}

		// 每批最多写入 200 条
		err = tx.
			Omit("User").
			CreateInBatches(&item.Members, 200).
			Error
		if err != nil {
			return fmt.Errorf("initialize todo member permissions: %w", err)
		}

		return nil
	})
}

// 只查询属于某个群组的todo
func (r *gormRepository) List(
	ctx context.Context,
	query ListQuery,
) ([]model.Todo, int64, error) {
	items := make([]model.Todo, 0)
	var total int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查询当前群成员表
		// 即使还保留着旧 TodoMember 授权，退群后也不能访问
		var memberCount int64
		if err := tx.
			Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id = ?",
				query.GroupID,
				query.UserID,
			).
			Count(&memberCount).
			Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}

		if memberCount == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		// 只统计当前群组的total
		if err := tx.
			Model(&model.Todo{}).
			Where("group_id = ?", query.GroupID).
			Count(&total).
			Error; err != nil {
			return fmt.Errorf("count group todos: %w", err)
		}

		offset := (query.Page - 1) * query.PageSize

		if err := tx.
			Model(&model.Todo{}).
			Where("group_id = ?", query.GroupID).
			Preload("CustomDates").
			Order("id DESC").
			Limit(query.PageSize).
			Offset(offset).
			Find(&items).
			Error; err != nil {
			return fmt.Errorf("list group todos: %w", err)
		}

		return nil
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ByIDForUser 查询详情前，先确认访问者仍属于 Todo 所在的群组。
func (r *gormRepository) ByIDForUser(ctx context.Context, id uint, userID uint) (model.Todo, error) {
	var item model.Todo
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先找到todo的id和group_id
		var target model.Todo
		err := tx.Select("id", "group_id").First(&target, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrTodoNotFound
		}
		if err != nil {
			return fmt.Errorf("find todo %d group: %w", id, err)
		}

		// 再确认访问者仍属于该群组
		var memberCount int64
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", target.GroupID, userID).
			Count(&memberCount).Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}
		if memberCount == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		// 群内所有成员可见，不按 viewer/editor 筛选；确认资格后再加载详情。
		if err := tx.
			Preload("CustomDates").
			Preload("Creator", func(db *gorm.DB) *gorm.DB {
				// id 用于匹配 CreatorID；只加载创建者的展示资料。
				return db.Select("id", "username", "suffix", "nickname", "avatar")
			}).
			First(&item, id).Error; err != nil {
			return fmt.Errorf("load todo %d detail: %w", id, err)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.Todo{}, err
	}
	return item, nil
}

// ByID 供现有 Patch 的业务校验使用，不包含访问资格检查。
func (r *gormRepository) ByID(ctx context.Context, id uint) (model.Todo, error) {
	var item model.Todo

	err := r.db.
		WithContext(ctx).
		//service需要自定义日期所以要preload
		Preload("CustomDates").
		First(&item, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Todo{}, apperrors.ErrTodoNotFound
	}

	if err != nil {
		return model.Todo{}, fmt.Errorf("find todo %d: %w", id, err)
	}

	return item, nil
}

func (r *gormRepository) Patch(
	ctx context.Context,
	id uint,
	userID uint,
	command PatchCommand,
) (model.Todo, error) {
	if userID == 0 {
		return model.Todo{}, apperrors.ErrGroupUnauthenticated
	}
	if id == 0 {
		return model.Todo{}, apperrors.ErrTodoNotFound
	}

	updates := map[string]any{
		//乐观锁
		"version": gorm.Expr("version + 1"),
	}
	if command.Title != nil {
		updates["title"] = *command.Title
	}
	if command.Content != nil {
		updates["content"] = *command.Content
	}
	if command.Color != nil {
		updates["color"] = *command.Color
	}
	if command.StartsAt != nil {
		updates["starts_at"] = *command.StartsAt
	}
	if command.RepeatMode != nil {
		updates["repeat_mode"] = *command.RepeatMode
	}
	if command.NotifyMode != nil {
		updates["notify_mode"] = *command.NotifyMode
	}

	db := r.db.WithContext(ctx)
	var item model.Todo

	err := db.Transaction(func(tx *gorm.DB) error {
		var target model.Todo
		//查群id和创建者id
		err := tx.Select("id", "group_id", "creator_id").First(&target, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrTodoNotFound
		}
		if err != nil {
			return fmt.Errorf("find todo %d: %w", id, err)
		}

		//creator需要在group内
		var cnt int64
		if err := tx.Model(&model.GroupMember{}).
			Where(
				"group_id = ? AND user_id = ?",
				target.GroupID,
				userID,
			).
			Count(&cnt).Error; err != nil {
			return fmt.Errorf("check group membership: %w", err)
		}
		if cnt == 0 {
			return apperrors.ErrGroupAccessDenied
		}

		//想改这个todo的人也要在group内，而且role要是editor
		//如果是creator就不用查这个
		if target.CreatorID != userID {
			var editorCount int64
			if err := tx.Model(&model.TodoMember{}).
				Where(
					"todo_id = ? AND user_id = ? AND role = ?",
					target.ID,
					userID,
					model.TodoEditor,
				).
				Count(&editorCount).Error; err != nil {
				return fmt.Errorf("check todo edit permission: %w", err)
			}

			if editorCount == 0 {
				return apperrors.ErrTodoEditDenied
			}
		}

		//更新
		result := tx.Model(&model.Todo{}).
			Where("id = ? AND version = ?", id, command.Version).
			Updates(updates)

		if result.Error != nil {
			return fmt.Errorf("patch todo %d: %w", id, result.Error)
		}
		if result.RowsAffected == 0 {
			// 之前已经确认 Todo 存在
			// 所以RowsAffected为0只能是版本冲突
			return apperrors.ErrTodoConcurrentUpdate
		}

		// 5. 请求携带 custom_dates 时，替换旧日期集合
		// 删掉tododate表记录
		if command.CustomDates != nil {
			if err := tx.Where("todo_id = ?", id).
				Delete(&model.TodoDate{}).Error; err != nil {
				return fmt.Errorf("delete custom dates: %w", err)
			}

			//重新写
			dates := make([]model.TodoDate, 0, len(*command.CustomDates))
			for _, date := range *command.CustomDates {
				dates = append(dates, model.TodoDate{
					TodoID: id,
					Date:   date,
				})
			}

			if len(dates) > 0 {
				if err := tx.Create(&dates).Error; err != nil {
					return fmt.Errorf("create custom dates: %w", err)
				}
			}
		}

		// 6. 读取更新后的 Todo
		//这里不改完成名单
		if err := tx.Preload("CustomDates").
			First(&item, id).Error; err != nil {
			return fmt.Errorf("load patched todo: %w", err)
		}

		return nil

	})
	if err != nil {
		return model.Todo{}, err
	}

	return item, nil
}

func (r *gormRepository) Delete(ctx context.Context, id uint, userID uint) error {
	return r.db.WithContext(ctx).
		Transaction(func(tx *gorm.DB) error {
			// 找到 Todo 的所属群和创建者。
			var item model.Todo
			err := tx.Select("id", "group_id", "creator_id").
				First(&item, id).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.ErrTodoNotFound
			}
			if err != nil {
				return fmt.Errorf("find todo before delete: %w", err)
			}

			// 创建者也必须仍在群内。
			var memberCount int64
			err = tx.Model(&model.GroupMember{}).
				Where(
					"group_id = ? AND user_id = ?",
					item.GroupID,
					userID,
				).
				Count(&memberCount).Error

			if err != nil {
				return fmt.Errorf("check group membership: %w", err)
			}
			if memberCount == 0 {
				return apperrors.ErrGroupAccessDenied
			}

			// 只有创建者才能删除
			if item.CreatorID != userID {
				return apperrors.ErrTodoDeleteDenied
			}

			result := tx.Delete(&model.Todo{}, id)
			if result.Error != nil {
				return fmt.Errorf("delete todo %d: %w", id, result.Error)
			}
			if result.RowsAffected == 0 {
				return apperrors.ErrTodoNotFound
			}

			return nil
		})
}

func (r *gormRepository) SetOccurrenceDone(
	ctx context.Context,
	todoID uint,
	userID uint,
	occursOn time.Time,
	done bool,
) error {
	occursOn = completionDate(occursOn)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		todo, err := loadTodoForCompletion(tx, todoID, userID)
		if err != nil {
			return err
		}
		completion := model.TodoCompletion{
			TodoID:   todo.ID,
			OccursOn: occursOn,
			Records:  make([]model.CompletionEntry, 0),
		}
		err = tx.Where("todo_id = ? AND occurs_on = ?", todo.ID, occursOn).First(&completion).Error
		//如果是其他错误就返回
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find todo completion: %w", err)
		}
		//找当前user的位置
		index := -1
		for i, record := range completion.Records {
			if record.UserID == userID {
				index = i
				break
			}
		}
		//如果是完成
		if done {
			//已经存在完成记录，直接返回
			if index >= 0 {
				return nil
			}

			//如果不存在完成记录
			//先确认这一天会发生这个todo
			exists, err := todoOccursOn(todo, occursOn)
			if err != nil {
				return err
			}
			if !exists {
				return apperrors.ErrTodoOccurrenceNotFound
			}
			//再插入一条完成记录
			completion.Records = append(
				completion.Records,
				model.CompletionEntry{
					UserID:      userID,
					CompletedAt: time.Now().UTC(),
				})
		} else {
			//如果是取消完成
			//如果不存在完成记录，直接返回
			if index < 0 {
				return nil
			}
			//只删掉自己的记录
			completion.Records = append(
				completion.Records[:index],
				completion.Records[index+1:]...,
			)
		}

		//如果前面没有找到记录，说明是第一次完成，直接创建
		if completion.ID == 0 {
			if err := tx.Omit("Todo").Create(&completion).Error; err != nil {
				return fmt.Errorf("create todo completion: %w", err)
			}
		} else {
			//如果前面有记录，就只能改records字段，不能改todo_id和occurs_on
			// 只更新 Records；空数组也必须保存。
			// 用结构体更新，让 GORM 的 JSON serializer 处理这个字段。
			if err := tx.Model(&completion).
				Select("Records").
				Updates(&completion).Error; err != nil {
				return fmt.Errorf("update todo completion records: %w", err)
			}
		}

		return nil
	})
}

// 一次性查询当前范围的完成记录
func (r *gormRepository) CompletionsInRange(
	ctx context.Context,
	todoIDs []uint,
	from time.Time,
	to time.Time,
) ([]model.TodoCompletion, error) {
	if len(todoIDs) == 0 {
		return []model.TodoCompletion{}, nil
	}

	var completions []model.TodoCompletion
	fromDate := from.Format(time.DateOnly)
	toDate := to.Format(time.DateOnly)

	err := r.db.WithContext(ctx).
		Where(
			"todo_id IN ? AND occurs_on >= ? AND occurs_on < ?",
			todoIDs,
			fromDate,
			toDate,
		).
		Find(&completions).
		Error
	if err != nil {
		return nil, fmt.Errorf("list todo completions in range: %w", err)
	}

	return completions, nil
}

func (r *gormRepository) GetOccurrenceCompletions(
	ctx context.Context,
	todoID uint,
	userID uint,
	occursOn time.Time,
) ([]CompletionUser, error) {
	occursOn = completionDate(occursOn)
	items := make([]CompletionUser, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//先查在不在群里
		if _, err := loadTodoForCompletion(tx, todoID, userID); err != nil {
			return err
		}
		var completion model.TodoCompletion
		//查有没有这个todo
		err := tx.Where("todo_id = ? AND occurs_on = ?", todoID, occursOn).
			First(&completion).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find todo completion: %w", err)
		}
		if len(completion.Records) == 0 {
			return nil
		}
		//只需要id即可，丢给后面的sql
		userIDs := make([]uint, 0, len(completion.Records))
		for _, record := range completion.Records {
			userIDs = append(userIDs, record.UserID)
		}
		var users []model.User
		if err := tx.Select("id", "username", "suffix", "nickname", "avatar").
			Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return fmt.Errorf("load completion users: %w", err)
		}
		// 查询返回的顺序不保证与 Records 一致，必须按 ID 匹配
		usersByID := make(map[uint]model.User, len(users))
		for _, user := range users {
			usersByID[user.ID] = user
		}
		for _, record := range completion.Records {
			user, found := usersByID[record.UserID]
			if !found {
				user = model.User{ID: record.UserID}
			}
			items = append(items, CompletionUser{User: user, CompletedAt: record.CompletedAt})
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// 在同一个事务中检查todo是否存在并且用户在不在群里
func loadTodoForCompletion(
	tx *gorm.DB,
	todoID uint,
	userID uint,
) (model.Todo, error) {
	if userID == 0 {
		return model.Todo{}, apperrors.ErrGroupUnauthenticated
	}
	if todoID == 0 {
		return model.Todo{}, apperrors.ErrTodoNotFound
	}
	var item model.Todo

	err := tx.Preload("CustomDates").First(&item, todoID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Todo{}, apperrors.ErrTodoNotFound
	}
	if err != nil {
		return model.Todo{}, fmt.Errorf("find completion todo: %w", err)
	}

	var memberCount int64
	err = tx.Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", item.GroupID, userID).
		Count(&memberCount).Error

	if err != nil {
		return model.Todo{}, fmt.Errorf("check group membership: %w", err)
	}
	if memberCount == 0 {
		return model.Todo{}, apperrors.ErrGroupAccessDenied
	}

	return item, nil
}

// 日期归零，方便比较
func completionDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// 判断某条todo在某一天是否会发生
func todoOccursOn(item model.Todo, date time.Time) (bool, error) {
	if item.StartsAt == nil {
		return false, nil
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return false, fmt.Errorf("load todo timezone: %w", err)
	}

	dateKey := date.Format(time.DateOnly)
	startsAt := item.StartsAt.In(location)

	switch item.RepeatMode {
	case model.RepeatOnce:
		return startsAt.Format(time.DateOnly) == dateKey, nil

	case model.RepeatCustom:
		for _, customDate := range item.CustomDates {
			if customDate.Date.Format(time.DateOnly) == dateKey {
				return true, nil
			}
		}
		return false, nil
	}

	option, err := recurrenceOption(startsAt, item.RepeatMode)
	if err != nil {
		return false, err
	}

	rule, err := rrule.NewRRule(option)
	if err != nil {
		return false, fmt.Errorf("build todo recurrence: %w", err)
	}

	year, month, day := date.Date()
	from := time.Date(year, month, day, 0, 0, 0, 0, location)
	to := from.AddDate(0, 0, 1)

	next := rule.After(from, true)
	return !next.IsZero() && next.Before(to), nil
}

// 在同一事务内聚合CalendarCandidates和CompletionsInRange
func (r *gormRepository) CalendarData(
	ctx context.Context,
	userID uint,
	from time.Time,
	to time.Time,
) ([]model.Todo, []model.TodoCompletion, error) {
	var items []model.Todo
	var completions []model.TodoCompletion

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		//使用repo，不用r.db
		//不能在这里调用 r.CalendarCandidates，否则它仍然使用外面的 r.db
		txRepo := &gormRepository{db: tx}

		var err error
		items, err = txRepo.CalendarCandidates(ctx, userID, from, to)
		if err != nil {
			return err
		}

		todoIDs := make([]uint, 0, len(items))
		for _, item := range items {
			todoIDs = append(todoIDs, item.ID)
		}

		completions, err = txRepo.CompletionsInRange(
			ctx,
			todoIDs,
			from,
			to,
		)
		return err

	}, &sql.TxOptions{ReadOnly: true}) //表示只读
	if err != nil {
		return nil, nil, err
	}
	return items, completions, nil
}
