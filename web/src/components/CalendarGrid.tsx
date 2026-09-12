import type { CalendarItem } from "../types";
type Day = { key: string; day: number; isCurrentMonth: boolean };
interface Props {
 days: Day[]; events: CalendarItem[];
 calendarPicking: boolean; customPicking: boolean; startDatePicking: boolean; customDateSet: Set<string>;
 isPastDate: (key: string) => boolean; isHoveredStartPattern: (day: Day) => boolean; isSelectedStartPattern: (day: Day) => boolean;
 pickingRepeat: string; selectionOutlinePath: string; onBeginSelection: (key: string, event: React.PointerEvent) => void;
 onContinueSelection: (key: string, disabled: boolean) => void; onKeyboardSelection: (key: string, event: React.KeyboardEvent) => void;
 onOpenDay: (key: string) => void; onOpenEvent: (item: CalendarItem) => void; onPointerLeave: () => void;
}
import { useLayoutEffect, useRef, useState } from "react";
import { layoutDay, occursOnDay } from "../lib/timeline";
import { TODAY_KEY, WEEKDAYS, calendarNote } from "../lib/calendar";

export default function CalendarGrid({
  days,
  events,
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
}: Props) {
  const grid = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({width: 100, height: 80});
  useLayoutEffect(() => {
    if (!grid.current) return;
    const observer = new ResizeObserver(([entry]) => setSize({width: entry.contentRect.width / 7, height: entry.contentRect.height / 6}));
    observer.observe(grid.current);
    return () => observer.disconnect();
  }, []);
  return (
    <>
      <div className="weekday-row" role="row">
        {WEEKDAYS.map((weekday) => (
          <div role="columnheader" key={weekday}>{weekday}</div>
        ))}
      </div>

      <div ref={grid} className="month-grid" role="grid" onPointerLeave={onPointerLeave}>
        {selectionOutlinePath && (
          <svg className="selection-outline" viewBox="0 0 7 6" preserveAspectRatio="none" aria-hidden="true">
            <path d={selectionOutlinePath} vectorEffect="non-scaling-stroke" />
          </svg>
        )}

        {days.map((day) => {
          const dayEvents = events
            .filter((item) => occursOnDay(item, day.key))
            .sort((left, right) => left.time.localeCompare(right.time));
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
                className="date-button" title={`${day.key} ${calendarNote(day.key)}`}
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

              <MonthCellItems items={dayEvents} date={day.key} size={size} picking={calendarPicking} onOpen={onOpenEvent} onMore={() => onOpenDay(day.key)} />
            </div>
          );
        })}
      </div>
    </>
  );
}

function MonthCellItems({ items, date, size, picking, onOpen, onMore }: {
 items: CalendarItem[]; date: string; size: {width: number; height: number}; picking: boolean;
 onOpen: (item: CalendarItem) => void; onMore: () => void;
}) {
 const todos = items.filter(item => item.kind === "todo");
 const columns = Math.max(1, Math.floor((size.width - 40) / 16));
 const half = size.height / 2;
 const todoRows = Math.max(1, Math.floor((half - 6) / 16));
 const todoOverflow = todos.length > todoRows * columns;
 const visibleTodoRows = Math.max(1, Math.floor((half - 6 - (todoOverflow ? 12 : 0)) / 16));
 const hiddenTodos = Math.max(0, todos.length - visibleTodoRows * columns);
 const segments = layoutDay(items, date, 100000).segments;
 const laneCount = Math.max(0, ...segments.map(segment => segment.track + 1));
 const eventRows = Math.max(1, Math.floor((half - 6) / 12));
 const eventOverflow = laneCount > eventRows;
 const visibleEventRows = Math.max(1, Math.floor((half - 6 - (eventOverflow ? 12 : 0)) / 12));
 const hiddenEvents = segments.filter(segment => segment.track >= visibleEventRows).length;
 return <div className="month-cell-items" aria-hidden={picking || undefined} inert={picking || undefined}>
   <div className="month-todo-half" aria-label={date + "待办"}>
     <div className="month-todo-clip" style={{height: visibleTodoRows * 16, gridTemplateColumns: 'repeat(' + columns + ', 16px)'}}>
       {todos.map(item => <button key={item.id} type="button" data-search-key={'todo-' + item.todoId} className={'month-todo-dot ' + (item.allDone || item.occurrenceDone ? 'is-done' : 'is-pending')} style={{color: item.color}} title={item.title + ' ' + item.time} aria-label={item.title + (item.allDone || item.occurrenceDone ? '，已完成' : '，未完成')} onClick={() => onOpen(item)}><span /></button>)}
     </div>
     {hiddenTodos > 0 && <button className="month-more" aria-label={date + '还有' + hiddenTodos + '条待办，查看当天'} onClick={onMore}>+{hiddenTodos}</button>}
   </div>
   <div className="month-event-half" aria-label={date + "日程"}>
     <div className="month-event-clip" style={{height: visibleEventRows * 12}}><div style={{position:'relative',height: laneCount * 12}}>
       {segments.map(({item,start,end,track}) => <button key={item.id} type="button" className="month-event-bar" data-track={track} data-search-key={'event-' + item.eventId} style={{top: track * 12, left: (start / 1440 * 100) + '%', width: 'max(8px, ' + ((end-start)/1440*100) + '%)', color: item.color}} title={item.title + ' · ' + item.date + ' ' + item.time + ' — ' + item.endDate + ' ' + item.endTime} aria-label={item.title + '，日程轨道' + (track+1)} onClick={() => onOpen(item)}><span /></button>)}
     </div></div>
     {hiddenEvents > 0 && <button className="month-more" aria-label={date + '还有' + hiddenEvents + '条日程，查看当天'} onClick={onMore}>+{hiddenEvents}</button>}
   </div>
 </div>;
}
