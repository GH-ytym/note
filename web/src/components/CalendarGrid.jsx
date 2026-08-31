import { DotsThree } from "@phosphor-icons/react";
import EventDot from "./EventDot";
import { TODAY_KEY, WEEKDAYS, calendarNote } from "../lib/calendar";

export default function CalendarGrid({
  days,
  events,
  expandedDayKey,
  setExpandedDayKey,
  calendarPicking,
  customPicking,
  startDatePicking,
  customDateSet,
  isPastDate,
  isHoveredStartPattern,
  isSelectedStartPattern,
  pickingRepeat,
  selectionOutlinePath,
  onBeginSelection,
  onContinueSelection,
  onKeyboardSelection,
  onOpenDay,
  onOpenEvent,
  onPointerLeave,
}) {
  return (
    <>
      <div className="weekday-row" role="row">
        {WEEKDAYS.map((weekday) => (
          <div role="columnheader" key={weekday}>{weekday}</div>
        ))}
      </div>

      <div className="month-grid" role="grid" onPointerLeave={onPointerLeave}>
        {selectionOutlinePath && (
          <svg className="selection-outline" viewBox="0 0 7 6" preserveAspectRatio="none" aria-hidden="true">
            <path d={selectionOutlinePath} vectorEffect="non-scaling-stroke" />
          </svg>
        )}

        {days.map((day) => {
          const dayEvents = events
            .filter((item) => item.date === day.key)
            .sort((left, right) => left.time.localeCompare(right.time));
          const isExpanded = expandedDayKey === day.key;
          const visibleEvents = dayEvents.length > 9 ? dayEvents.slice(0, 8) : dayEvents.slice(0, 9);
          const selectedCustomDate = customDateSet.has(day.key);
          const pastDateDisabled = calendarPicking && isPastDate(day.key);
          const hoveredStartPattern = isHoveredStartPattern(day);
          const selectedStartPattern = isSelectedStartPattern(day);
          const selectedStartDate = !["每周", "每月"].includes(pickingRepeat) && selectedStartPattern;
          const selectedRepeatPattern = ["每周", "每月"].includes(pickingRepeat) && selectedStartPattern;

          return (
            <div
              className={[
                "day-cell",
                day.isCurrentMonth ? "" : "is-adjacent",
                day.key === TODAY_KEY ? "is-today" : "",
                selectedCustomDate ? "is-custom-selected" : "",
                hoveredStartPattern ? "is-start-highlighted" : "",
                selectedStartDate ? "is-start-selected" : "",
                selectedRepeatPattern ? "is-pattern-selected" : "",
                calendarPicking && !pastDateDisabled ? "is-pickable" : "",
                pastDateDisabled ? "is-past-disabled" : "",
              ].filter(Boolean).join(" ")}
              role="gridcell"
              key={day.key}
              onPointerDown={(event) => onBeginSelection(day.key, event)}
              onPointerEnter={() => onContinueSelection(day.key, pastDateDisabled)}
              onPointerMove={() => onContinueSelection(day.key, pastDateDisabled)}
            >
              <button
                className="date-button"
                type="button"
                disabled={pastDateDisabled}
                aria-label={pastDateDisabled
                  ? `${day.key}已过期，不可选择`
                  : customPicking
                    ? `${selectedCustomDate ? "取消" : "选择"}${day.key}`
                    : startDatePicking
                      ? `选择开始日期${day.key}`
                      : `查看${day.key}日程`}
                aria-pressed={calendarPicking ? (customPicking ? selectedCustomDate : selectedStartPattern) : undefined}
                onKeyDown={(event) => onKeyboardSelection(day.key, event)}
                onClick={() => {
                  if (!calendarPicking) onOpenDay(day.key);
                }}
              >
                <span className="date-mark"><strong>{day.day}</strong></span>
                <small className="date-note">{calendarNote(day.key)}</small>
              </button>

              <div className="day-events" aria-label={`${day.key}日程`} aria-hidden={calendarPicking ? "true" : undefined} inert={calendarPicking ? true : undefined}>
                {visibleEvents.map((item) => (
                  <EventDot item={item} onSelect={onOpenEvent} key={item.id} />
                ))}

                {dayEvents.length > 9 && (
                  <button
                    className="event-more"
                    type="button"
                    aria-label={`展开${dayEvents.length}条日程`}
                    aria-expanded={isExpanded}
                    onClick={(event) => {
                      event.stopPropagation();
                      setExpandedDayKey(isExpanded ? null : day.key);
                    }}
                  >
                    <DotsThree size={17} weight="bold" />
                  </button>
                )}

                {isExpanded && (
                  <div className="event-overflow" onPointerDown={(event) => event.stopPropagation()}>
                    {dayEvents.map((item) => (
                      <EventDot item={item} onSelect={onOpenEvent} key={`expanded-${item.id}`} />
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </>
  );
}
