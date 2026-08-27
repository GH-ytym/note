package todo

import (
	"context"
	"fmt"
	"note/internal/model"
	"sort"
	"time"

	"github.com/teambition/rrule-go"
)

func (s *service) CalendarOccurrences(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]CalendarOccurrence, error) {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return nil, ErrInvalidCalendarRange
	}

	//找到所有的todo候选
	items, err := s.repo.CalendarCandidates(ctx, from, to)
	if err != nil {
		return nil, err
	}

	occurrences := make([]CalendarOccurrence, 0)

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if item.StartsAt == nil {
			continue
		}

		// PostgreSQL 的 timestamptz 可能以 UTC 返回。
		// 周期计算必须使用日历查询所在的本地时区。
		startsAt := item.StartsAt.In(from.Location())

		// 仅一次的 Todo 不需要周期计算器。
		if item.RepeatMode == model.RepeatOnce {
			occurrences = append(occurrences, CalendarOccurrence{
				TodoID:     item.ID,
				Content:    item.Content,
				Color:      item.Color,
				Done:       item.Done,
				StartsAt:   startsAt,
				OccursAt:   startsAt,
				RepeatMode: item.RepeatMode,
				NotifyMode: notifyModeOf(item),
				Version:    item.Version,
			})
			continue
		}

		//custom也不需要
		if item.RepeatMode == model.RepeatCustom {
			for _, date := range item.CustomDates {
				year, month, day := date.Date.Date()
				occursAt := time.Date(
					year,
					month,
					day,
					startsAt.Hour(),
					startsAt.Minute(),
					startsAt.Second(),
					startsAt.Nanosecond(),
					from.Location(),
				)
				if occursAt.Before(from) || !occursAt.Before(to) {
					continue
				}
				occurrences = append(occurrences, CalendarOccurrence{
					TodoID:     item.ID,
					Content:    item.Content,
					Color:      item.Color,
					Done:       item.Done,
					StartsAt:   startsAt,
					OccursAt:   occursAt,
					RepeatMode: item.RepeatMode,
					NotifyMode: notifyModeOf(item),
					Version:    item.Version,
				})
			}
			continue
		}

		//用rrule解析自己设定的repeatmode
		option, err := recurrenceOption(
			startsAt,
			item.RepeatMode,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"build recurrence option for todo %d: %w",
				item.ID,
				err,
			)
		}

		//生成rule规则
		rule, err := rrule.NewRRule(option)
		if err != nil {
			return nil, fmt.Errorf(
				"build recurrence rule for todo %d: %w",
				item.ID,
				err,
			)
		}

		//算出一个todo在[from,to)内的所有发生日期
		//找不到刚好就不用append
		times := rule.Between(from, to, true)

		//遍历日期加入occurrences
		for _, occursAt := range times {
			// Between 的 true 会同时包含 from 和 to。
			// 我们需要 [from, to)，所以排除恰好等于 to 的实例。
			if !occursAt.Before(to) {
				continue
			}

			occurrences = append(occurrences, CalendarOccurrence{
				TodoID:     item.ID,
				Content:    item.Content,
				Color:      item.Color,
				Done:       item.Done,
				StartsAt:   startsAt,
				OccursAt:   occursAt,
				RepeatMode: item.RepeatMode,
				NotifyMode: notifyModeOf(item),
				Version:    item.Version,
			})
		}
	}

	//按时间排序
	sort.Slice(occurrences, func(i, j int) bool {
		return occurrences[i].OccursAt.Before(
			occurrences[j].OccursAt,
		)
	})

	return occurrences, nil
}

func notifyModeOf(item model.Todo) *model.NotifyMode {
	if item.Reminder == nil || !item.Reminder.Enabled {
		return nil
	}

	notifyMode := item.Reminder.NotifyMode
	return &notifyMode
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
		return rrule.ROption{}, ErrInvalidRepeatMode
	}

	return option, nil
}
