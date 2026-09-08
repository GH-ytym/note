package todo

import (
	"context"
	"strings"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/utils"
)

// TodoService 声明 Todo 对外提供的业务操作。
type TodoService interface {
	Create(ctx context.Context, command CreateCommand) (model.Todo, error)
	List(ctx context.Context, query ListQuery) (Page, error)
	Get(ctx context.Context, id uint) (model.Todo, error)
	Patch(ctx context.Context, id uint, command PatchCommand) (model.Todo, error)
	Delete(ctx context.Context, id uint) error
	CalendarOccurrences(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]CalendarOccurrence, error)
	SetOccurrenceDone(
		ctx context.Context,
		todoID uint,
		occursOn time.Time,
		done bool,
	) error
}

// service 负责执行业务规则，并通过 Repository 完成数据持久化。
// 具体实现对 todo 包外隐藏。
type service struct {
	repo Repository
}

func NewService(repo Repository) TodoService {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, command CreateCommand) (model.Todo, error) {
	//处理在handler组装的command
	title := strings.TrimSpace(command.Title)
	if title == "" {
		return model.Todo{}, apperrors.ErrTitleRequired
	}

	//没传 content       → content = title
	//传了 null          → content = title
	//传了空字符串        → content = title
	//传了正常文字        → 使用这段文字
	content := title
	if command.Content != nil {
		content1 := strings.TrimSpace(*command.Content)
		if content1 != "" {
			content = content1
		}
	}

	color := command.Color
	if color == "" {
		color = utils.RandomColor()
	}
	color, validColor := utils.NormalizeHexColor(color)
	if !validColor {
		return model.Todo{}, apperrors.ErrTodoInvalidColor
	}

	if !validRepeatMode(command.RepeatMode) {
		return model.Todo{}, apperrors.ErrInvalidRepeatMode
	}
	// 每个 Todo 都必须从一个明确的日期和时间开始。
	if command.StartsAt == nil {
		return model.Todo{}, apperrors.ErrTodoStartsAtRequired
	}

	//解析提醒模式
	notifyMode := command.NotifyMode
	if notifyMode == "" {
		notifyMode = model.NotifyNone
	}
	if !validNotifyMode(notifyMode) {
		return model.Todo{}, apperrors.ErrInvalidNotifyMode
	}

	//必须是custom才能组装customDates
	if command.RepeatMode == model.RepeatCustom {
		if len(command.CustomDates) == 0 {
			return model.Todo{}, apperrors.ErrCustomDatesRequired
		}
	} else if len(command.CustomDates) > 0 {
		return model.Todo{}, apperrors.ErrCustomDatesNotAllowed
	}

	dates := make([]model.TodoDate, 0, len(command.CustomDates))
	seenDates := make(map[string]struct{}, len(command.CustomDates))
	for _, date := range command.CustomDates {
		dateKey := date.Format(time.DateOnly)
		if _, exists := seenDates[dateKey]; exists {
			continue
		}
		seenDates[dateKey] = struct{}{}

		dates = append(dates, model.TodoDate{
			Date: date,
		})
	}

	item := model.Todo{
		Title:       title,
		Content:     &content,
		Color:       color,
		StartsAt:    command.StartsAt,
		RepeatMode:  command.RepeatMode,
		NotifyMode:  notifyMode,
		CustomDates: dates,
	}

	//进入Repository的Create
	if err := s.repo.Create(ctx, &item); err != nil {
		return model.Todo{}, err
	}

	return item, nil
}

func (s *service) List(ctx context.Context, query ListQuery) (Page, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return Page{}, apperrors.ErrInvalidPagination
	}

	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return Page{}, err
	}

	return Page{
		Items:    items,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}, nil
}

func (s *service) Get(ctx context.Context, id uint) (model.Todo, error) {
	return s.repo.ByID(ctx, id)
}

func (s *service) Patch(
	ctx context.Context,
	id uint,
	command PatchCommand,
) (model.Todo, error) {
	// 乐观锁必须携带版本号。
	if command.Version == 0 {
		return model.Todo{}, apperrors.ErrTodoInvalidVersion
	}

	//啥都没改则不进入repo防止version自增
	if command.Title == nil &&
		command.Content == nil &&
		command.Color == nil &&
		command.NotifyMode == nil &&
		command.StartsAt == nil &&
		command.RepeatMode == nil &&
		command.AllDone == nil &&
		command.CustomDates == nil {
		return model.Todo{}, apperrors.ErrNothingToUpdate
	}

	//处理command字段
	if command.Title != nil {
		title := strings.TrimSpace(*command.Title)
		if title == "" {
			return model.Todo{}, apperrors.ErrTitleRequired
		}
		command.Title = &title
	}

	//先留content，把旧title一起查出来再校验
	if command.Content != nil {
		content := strings.TrimSpace(*command.Content)
		command.Content = &content
	}

	if command.Color != nil {
		color, validColor := utils.NormalizeHexColor(*command.Color)
		if !validColor {
			return model.Todo{}, apperrors.ErrTodoInvalidColor
		}

		command.Color = &color
	}

	if command.RepeatMode != nil &&
		!validRepeatMode(*command.RepeatMode) {
		return model.Todo{}, apperrors.ErrInvalidRepeatMode
	}

	if command.NotifyMode != nil && !validNotifyMode(*command.NotifyMode) {
		return model.Todo{}, apperrors.ErrInvalidNotifyMode
	}

	//先找到改之前的这条todo
	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return model.Todo{}, err
	}

	if command.Content != nil && *command.Content == "" {
		finalTitle := current.Title

		if command.Title != nil {
			finalTitle = *command.Title
		}
		//如果不传新title但把content传成空了，就沿用旧title
		command.Content = &finalTitle
	}

	// 没传 repeat_mode，就继续使用数据库里的旧模式。
	finalRepeatMode := current.RepeatMode
	//传了就改模式
	if command.RepeatMode != nil {
		finalRepeatMode = *command.RepeatMode
	}

	// 去重自定义日期
	if command.CustomDates != nil {
		dates := utils.UniqueDates(*command.CustomDates)
		command.CustomDates = &dates
	}

	//处理自定义模式
	//finalRepeatMode可能是沿用之前的custom，也可能是改到了custom
	if finalRepeatMode == model.RepeatCustom {
		switch {
		// 本次请求传了 custom_dates，就校验新的日期集合。
		case command.CustomDates != nil:
			if len(*command.CustomDates) == 0 {
				return model.Todo{}, apperrors.ErrCustomDatesRequired
			}
		// 原来不是custom就没有日期可用
		//或者原来的日期为空，总之没有可以继承的旧日期
		case current.RepeatMode != model.RepeatCustom || len(current.CustomDates) == 0:
			return model.Todo{}, apperrors.ErrCustomDatesRequired
		}
	} else {
		//处理非自定义模式
		// 非 custom 模式不能携带自定义日期。
		if command.CustomDates != nil && len(*command.CustomDates) > 0 {
			return model.Todo{}, apperrors.ErrCustomDatesNotAllowed
		}

		// 从 custom 切换到其他模式时，主动清空旧日期。
		if current.RepeatMode == model.RepeatCustom {
			emptyDates := []time.Time{}
			command.CustomDates = &emptyDates
		}
	}

	return s.repo.Patch(ctx, id, command)
}

func (s *service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) SetOccurrenceDone(
	ctx context.Context,
	todoID uint,
	occursOn time.Time,
	done bool,
) error {
	//先保证todo存在
	if _, err := s.repo.ByID(ctx, todoID); err != nil {
		return err
	}
	// 只插入或删除一条 todo_completions。
	return s.repo.SetOccurrenceDone(
		ctx,
		todoID,
		occursOn,
		done,
	)
}

// 两个辅助校验函数
func validRepeatMode(mode model.RepeatMode) bool {
	switch mode {
	case model.RepeatOnce,
		model.RepeatDaily,
		model.RepeatWeekdays,
		model.RepeatWeekends,
		model.RepeatWeekly,
		model.RepeatMonthly,
		model.RepeatCustom:
		return true
	default:
		return false
	}
}

func validNotifyMode(mode model.NotifyMode) bool {
	switch mode {
	case model.NotifyNone,
		model.NotifySilent,
		model.NotifyPopup:
		return true
	default:
		return false
	}
}
