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

// Service declares the Event operations available to the HTTP layer.
type EventService interface {
	Create(ctx context.Context, command CreateCommand) (model.Event, error)
	ListInRange(ctx context.Context, from, to time.Time) ([]CalendarOccurrence, error)
	Get(ctx context.Context, id uint) (model.Event, error)
	Patch(ctx context.Context, id uint, command PatchCommand) (model.Event, error)
}

func (s *service) Get(ctx context.Context, id uint) (model.Event, error) { return s.repo.Get(ctx, id) }

func (s *service) Patch(ctx context.Context, id uint, command PatchCommand) (model.Event, error) {
	if command.Version == 0 {
		return model.Event{}, apperrors.ErrEventInvalidVersion
	}
	if command.Title == nil && command.Content == nil && command.StartsAt == nil && command.EndsAt == nil {
		return model.Event{}, apperrors.ErrNothingToUpdate
	}
	item, err := s.repo.Get(ctx, id)
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
	if err := s.repo.Update(ctx, &item, command.Version); err != nil {
		return model.Event{}, err
	}
	return item, nil
}

type service struct {
	repo EventRepository
}

func NewService(repo EventRepository) EventService {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, command CreateCommand) (model.Event, error) {
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

func (s *service) ListInRange(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]CalendarOccurrence, error) {
	if from.IsZero() ||
		to.IsZero() ||
		!from.Before(to) {
		return nil, apperrors.ErrInvalidCalendarRange
	}
	//找到所有可能的event集合
	items, err := s.repo.ListInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	occurrences := make([]CalendarOccurrence, 0)
	for _, item := range items {
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
					EventID:    item.ID,
					Title:      item.Title,
					Content:    content,
					Color:      item.Color,
					StartsAt:   startsat,
					EndsAt:     endsat,
					RepeatMode: item.RepeatMode,
					Version:    item.Version,
				})
			}
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
					EventID:    item.ID,
					Title:      item.Title,
					Content:    content,
					Color:      item.Color,
					StartsAt:   occursAt,
					EndsAt:     endsAt,
					RepeatMode: item.RepeatMode,
					Version:    item.Version,
				})
			}
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
				EventID: item.ID,
				Title:   item.Title,
				Content: content,
				Color:   item.Color,
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
