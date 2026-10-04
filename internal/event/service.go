package event

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/utils"

	"github.com/teambition/rrule-go"
)

// Service 声明 Event 对外提供的业务操作。
type Service interface {
	Create(ctx context.Context, command CreateCommand) (model.Event, error)
	List(ctx context.Context, query ListQuery) (Page, error)
	ListInRange(ctx context.Context, userID uint, from, to time.Time) ([]CalendarOccurrence, error)
	Get(ctx context.Context, id, userID uint) (model.Event, error)
	Patch(ctx context.Context, id, userID uint, command PatchCommand) (model.Event, error)
	Delete(ctx context.Context, id, userID uint) error
	PatchRole(ctx context.Context, actorID, eventID uint, userIDs []uint, role model.EventRole) error
}

// service 负责执行业务规则，并通过 Repository 完成数据持久化。
type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Get(ctx context.Context, id, userID uint) (model.Event, error) {
	if userID == 0 {
		return model.Event{}, apperrors.ErrGroupUnauthenticated
	}
	if id == 0 {
		return model.Event{}, apperrors.ErrEventNotFound
	}
	return s.repo.Get(ctx, id, userID)
}

func (s *service) Patch(ctx context.Context, id, userID uint, command PatchCommand) (model.Event, error) {
	if userID == 0 {
		return model.Event{}, apperrors.ErrGroupUnauthenticated
	}
	if id == 0 {
		return model.Event{}, apperrors.ErrEventNotFound
	}
	if command.Version == 0 {
		return model.Event{}, apperrors.ErrEventInvalidVersion
	}
	if command.Title == nil && command.Content == nil && command.StartsAt == nil && command.EndsAt == nil {
		return model.Event{}, apperrors.ErrNothingToUpdate
	}
	item, err := s.repo.Get(ctx, id, userID)
	if err != nil {
		return model.Event{}, err
	}
	if item.Version != command.Version {
		return model.Event{}, apperrors.ErrEventConcurrentUpdate
	}
	if command.Title != nil {
		item.Title = strings.TrimSpace(*command.Title)
		if item.Title == "" {
			return model.Event{}, apperrors.ErrTitleRequired
		}
	}
	if command.Content != nil {
		content := strings.TrimSpace(*command.Content)
		if len([]rune(content)) > 500 {
			return model.Event{}, apperrors.ErrEventInvalidContent
		}
		item.Content = &content
	}
	if command.StartsAt != nil {
		item.StartsAt = *command.StartsAt
	}
	if command.EndsAt != nil {
		item.EndsAt = *command.EndsAt
	}
	if item.StartsAt.IsZero() || item.EndsAt.IsZero() || !item.EndsAt.After(item.StartsAt) {
		return model.Event{}, apperrors.ErrInvalidEventTimeRange
	}
	// 写事务里再次检查当前群成员和编辑权限，避免校验后退群或被降权。
	if err := s.repo.Update(ctx, &item, command.Version, userID); err != nil {
		return model.Event{}, err
	}
	return item, nil
}

func (s *service) Create(ctx context.Context, command CreateCommand) (model.Event, error) {
	if command.CreatorID == 0 {
		return model.Event{}, apperrors.ErrGroupUnauthenticated
	}
	if command.GroupID == 0 {
		return model.Event{}, apperrors.ErrEventInvalidGroup
	}
	title := strings.TrimSpace(command.Title)
	if title == "" {
		return model.Event{}, apperrors.ErrTitleRequired
	}

	var content *string
	if command.Content != nil {
		trimmedContent := strings.TrimSpace(*command.Content)
		if len([]rune(trimmedContent)) > 500 {
			return model.Event{}, apperrors.ErrEventInvalidContent
		}
		content = &trimmedContent
	}

	color := command.Color
	if color == "" {
		color = utils.RandomColor()
	}
	color1, valid := utils.NormalizeHexColor(color)
	if !valid {
		return model.Event{}, apperrors.ErrEventInvalidColor
	}

	//时间校验
	if command.StartsAt.IsZero() ||
		command.EndsAt.IsZero() ||
		!command.EndsAt.After(command.StartsAt) {
		return model.Event{}, apperrors.ErrInvalidEventTimeRange
	}

	if !validRepeatMode(command.RepeatMode) {
		return model.Event{}, apperrors.ErrInvalidRepeatMode
	}
	if command.RepeatMode == model.RepeatCustom {
		if len(command.CustomDates) == 0 {
			return model.Event{}, apperrors.ErrCustomDatesRequired
		}
	} else if len(command.CustomDates) > 0 {
		return model.Event{}, apperrors.ErrCustomDatesNotAllowed
	}

	uniqueDates := utils.UniqueDates(command.CustomDates)
	customDates := make([]model.EventDate, 0, len(uniqueDates))
	for _, date := range uniqueDates {
		customDates = append(customDates, model.EventDate{Date: date})
	}
	item := model.Event{
		GroupID:     command.GroupID,
		CreatorID:   command.CreatorID,
		Title:       title,
		Content:     content,
		Color:       color1,
		StartsAt:    command.StartsAt,
		EndsAt:      command.EndsAt,
		RepeatMode:  command.RepeatMode,
		CustomDates: customDates,
	}
	if err := s.repo.Create(ctx, &item); err != nil {
		return model.Event{}, err
	}
	return item, nil
}

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

func (s *service) List(ctx context.Context, query ListQuery) (Page, error) {
	if query.UserID == 0 {
		return Page{}, apperrors.ErrGroupUnauthenticated
	}
	if query.GroupID == 0 {
		return Page{}, apperrors.ErrEventInvalidGroup
	}
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || query.Page-1 > int(^uint(0)>>1)/query.PageSize {
		return Page{}, apperrors.ErrInvalidPagination
	}
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *service) Delete(ctx context.Context, id, userID uint) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if id == 0 {
		return apperrors.ErrEventNotFound
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *service) PatchRole(ctx context.Context, actorID, eventID uint, userIDs []uint, role model.EventRole) error {
	if actorID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	if eventID == 0 {
		return apperrors.ErrEventNotFound
	}
	if (role != model.EventViewer && role != model.EventEditor) || len(userIDs) == 0 || len(userIDs) > 100 {
		return apperrors.ErrEventRoleInvalid
	}
	ids := make([]uint, 0, len(userIDs))
	seen := make(map[uint]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			return apperrors.ErrEventRoleInvalid
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return s.repo.PatchRole(ctx, actorID, eventID, ids, role)
}

func (s *service) ListInRange(
	ctx context.Context,
	userID uint,
	from time.Time,
	to time.Time,
) ([]CalendarOccurrence, error) {
	if userID == 0 {
		return nil, apperrors.ErrGroupUnauthenticated
	}
	if from.IsZero() ||
		to.IsZero() ||
		!from.Before(to) {
		return nil, apperrors.ErrInvalidCalendarRange
	}
	//找到所有可能的event集合
	items, err := s.repo.ListInRange(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	occurrences := make([]CalendarOccurrence, 0)
	for _, item := range items {
		myRole := model.EventViewer
		if item.CreatorID == userID {
			myRole = model.EventEditor
		}
		for _, member := range item.Members {
			if member.UserID == userID && item.CreatorID != userID {
				myRole = member.Role
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if item.StartsAt.IsZero() || item.EndsAt.IsZero() || !item.EndsAt.After(item.StartsAt) {
			return nil, fmt.Errorf("event %d: %w", item.ID, apperrors.ErrInvalidEventTimeRange)
		}
		//与todo类似
		//先不拿出endsat
		startsat := item.StartsAt.In(from.Location())
		duration := item.EndsAt.Sub(item.StartsAt)

		//调整content
		content := ""
		if item.Content != nil {
			content = *item.Content
		}

		//1.once
		if item.RepeatMode == model.RepeatOnce {
			endsat := startsat.Add(duration)
			//和repo一样的相交规则
			if startsat.Before(to) && endsat.After(from) {
				occurrences = append(occurrences, CalendarOccurrence{
					MyRole:     myRole,
					EventID:    item.ID,
					GroupID:    item.GroupID,
					CreatorID:  item.CreatorID,
					Title:      item.Title,
					Content:    content,
					Color:      item.Color,
					StartsAt:   startsat,
					EndsAt:     endsat,
					RepeatMode: item.RepeatMode,
					Version:    item.Version,
				})
			}
			//当前类型已经判断出来了，对于这个event就不往下判断了
			continue
		}

		// 2. custom：使用自定义日期和原始时分秒组成每次开始时间。
		if item.RepeatMode == model.RepeatCustom {
			for _, customDate := range item.CustomDates {
				year, month, day := customDate.Date.Date()
				occursAt := time.Date(
					year,
					month,
					day,
					startsat.Hour(),
					startsat.Minute(),
					startsat.Second(),
					startsat.Nanosecond(),
					from.Location(),
				)
				endsAt := occursAt.Add(duration)
				if !occursAt.Before(to) || !endsAt.After(from) {
					continue
				}
				occurrences = append(occurrences, CalendarOccurrence{
					MyRole:     myRole,
					EventID:    item.ID,
					GroupID:    item.GroupID,
					CreatorID:  item.CreatorID,
					Title:      item.Title,
					Content:    content,
					Color:      item.Color,
					StartsAt:   occursAt,
					EndsAt:     endsAt,
					RepeatMode: item.RepeatMode,
					Version:    item.Version,
				})
			}
			//同理，直接跳过剩下的
			continue
		}
		// 3. 普通周期
		//和todo一样先构造周期规则
		option, err := recurrenceOption(startsat, item.RepeatMode)
		if err != nil {
			return nil, err
		}
		rule, err := rrule.NewRRule(option)
		if err != nil {
			return nil, err
		}
		//由于有持续时间，查询需要向前加个duration
		from1 := from.Add(-duration)
		times := rule.Between(from1, to, true)
		for _, t := range times {
			//t是某个周期event的某次开始时间
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			//当前
			et := t.Add(duration)
			//一样的相交规则，不符合则跳过
			if !t.Before(to) || !et.After(from) {
				continue
			}
			occurrences = append(occurrences, CalendarOccurrence{
				MyRole:    myRole,
				EventID:   item.ID,
				GroupID:   item.GroupID,
				CreatorID: item.CreatorID,
				Title:     item.Title,
				Content:   content,
				Color:     item.Color,
				//某次的开始时间，不是最初的startsat
				StartsAt: t,
				//某次的结束时间，不是在最初的endsat
				EndsAt:     et,
				RepeatMode: item.RepeatMode,
				Version:    item.Version,
			})
		}
	}
	//统一排序
	sort.Slice(occurrences, func(i, j int) bool {
		//如果开始时间相同，id小的排前面
		if occurrences[i].StartsAt.Equal(occurrences[j].StartsAt) {
			return occurrences[i].EventID < occurrences[j].EventID
		}
		//时间不相同，更早开始的排前面
		return occurrences[i].StartsAt.Before(occurrences[j].StartsAt)
	})
	return occurrences, nil
}

func recurrenceOption(
	startsAt time.Time,
	mode model.RepeatMode,
) (rrule.ROption, error) {
	option := rrule.ROption{
		Dtstart:  startsAt,
		Interval: 1,
	}

	switch mode {
	case model.RepeatDaily:
		option.Freq = rrule.DAILY

	case model.RepeatWeekdays:
		option.Freq = rrule.WEEKLY
		option.Byweekday = []rrule.Weekday{
			rrule.MO,
			rrule.TU,
			rrule.WE,
			rrule.TH,
			rrule.FR,
		}

	case model.RepeatWeekends:
		option.Freq = rrule.WEEKLY
		option.Byweekday = []rrule.Weekday{
			rrule.SA,
			rrule.SU,
		}

	case model.RepeatWeekly:
		option.Freq = rrule.WEEKLY

	case model.RepeatMonthly:
		option.Freq = rrule.MONTHLY

	default:
		return rrule.ROption{}, apperrors.ErrInvalidRepeatMode
	}

	return option, nil
}
