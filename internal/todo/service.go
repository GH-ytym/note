package todo

import (
	"context"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"note/internal/model"
)

var (
	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	eventColors     = []string{
		"#F3B51B", "#F47C48", "#E95B78", "#A879F2",
		"#5B8DEF", "#35B7A0", "#82B94B", "#D4A373",
	}
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
	content := strings.TrimSpace(command.Content)
	if content == "" {
		return model.Todo{}, ErrInvalidContent
	}

	color := command.Color
	if color == "" {
		color = eventColors[rand.IntN(len(eventColors))]
	}
	if !hexColorPattern.MatchString(color) {
		return model.Todo{}, ErrInvalidColor
	}
	color = strings.ToUpper(color)

	if !validRepeatMode(command.RepeatMode) {
		return model.Todo{}, ErrInvalidRepeatMode
	}
	// 每个 Todo 都必须从一个明确的日期和时间开始。
	if command.StartsAt == nil {
		return model.Todo{}, ErrStartsAtRequired
	}

	//解析提醒模式
	notifyMode := command.NotifyMode
	if notifyMode == "" {
		notifyMode = model.NotifyNone
	}
	if !validNotifyMode(notifyMode) {
		return model.Todo{}, ErrInvalidNotifyMode
	}

	//必须是custom才能组装customDates
	if command.RepeatMode == model.RepeatCustom {
		if len(command.CustomDates) == 0 {
			return model.Todo{}, ErrCustomDatesRequired
		}
	} else if len(command.CustomDates) > 0 {
		return model.Todo{}, ErrCustomDatesNotAllowed
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
		Content:     content,
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
		return Page{}, ErrInvalidPagination
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
		return model.Todo{}, ErrInvalidVersion
	}

	//啥都没改则不进入repo防止version自增
	if command.Content == nil &&
		command.Color == nil &&
		command.NotifyMode == nil &&
		command.StartsAt == nil &&
		command.RepeatMode == nil &&
		command.AllDone == nil &&
		command.CustomDates == nil {
		return model.Todo{}, ErrNothingToUpdate
	}

	//处理command字段
	if command.Content != nil {
		content := strings.TrimSpace(*command.Content)
		if content == "" {
			return model.Todo{}, ErrInvalidContent
		}
		command.Content = &content
	}
	if command.Color != nil {
		color := strings.ToUpper(*command.Color)

		if !hexColorPattern.MatchString(color) {
			return model.Todo{}, ErrInvalidColor
		}

		command.Color = &color
	}

	if command.RepeatMode != nil &&
		!validRepeatMode(*command.RepeatMode) {
		return model.Todo{}, ErrInvalidRepeatMode
	}

	if command.NotifyMode != nil && !validNotifyMode(*command.NotifyMode) {
		return model.Todo{}, ErrInvalidNotifyMode
	}

	//先找到改之前的这条todo
	current, err := s.repo.ByID(ctx, id)
	if err != nil {
		return model.Todo{}, err
	}

	// 没传 repeat_mode，就继续使用数据库里的旧模式。
	finalRepeatMode := current.RepeatMode
	//传了就改模式
	if command.RepeatMode != nil {
		finalRepeatMode = *command.RepeatMode
	}

	// 去重自定义日期
	if command.CustomDates != nil {
		dates := uniqueDates(*command.CustomDates)
		command.CustomDates = &dates
	}

	//处理自定义模式
	//finalRepeatMode可能是沿用之前的custom，也可能是改到了custom
	if finalRepeatMode == model.RepeatCustom {
		switch {
		// 本次请求传了 custom_dates，就校验新的日期集合。
		case command.CustomDates != nil:
			if len(*command.CustomDates) == 0 {
				return model.Todo{}, ErrCustomDatesRequired
			}
		// 原来不是custom就没有日期可用
		//或者原来的日期为空，总之没有可以继承的旧日期
		case current.RepeatMode != model.RepeatCustom || len(current.CustomDates) == 0:
			return model.Todo{}, ErrCustomDatesRequired
		}
	} else {
		//处理非自定义模式
		// 非 custom 模式不能携带自定义日期。
		if command.CustomDates != nil && len(*command.CustomDates) > 0 {
			return model.Todo{}, ErrCustomDatesNotAllowed
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

// 时间去重
func uniqueDates(values []time.Time) []time.Time {
	dates := make([]time.Time, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, date := range values {
		key := date.Format(time.DateOnly)

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		dates = append(dates, date)
	}

	return dates
}
