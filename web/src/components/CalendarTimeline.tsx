import type { CalendarItem } from "../types";
interface TimelineProps { days: string[]; items: CalendarItem[]; mode: string; orientation?: string; trackCount?: number; onDay: (date: string) => void; onTodo: (item: CalendarItem) => void; saving: boolean; onRetime: (item: CalendarItem, edge: "start" | "end", minutes: number) => Promise<void> }
import { useEffect, useRef, useState } from "react";
import {
  WEEKDAYS,
  dateFromKey,
  colorWithAlpha,
} from "../lib/calendar";
import { layoutDay } from "../lib/timeline";
import { layoutTimelineTodos } from "../lib/timeline-todos";
import useTimelineDrag from "../hooks/useTimelineDrag";
import useNow from "../hooks/useNow";

export default function CalendarTimeline({
  days,
  items,
  mode,
  orientation = "vertical",
  trackCount = 7,
  onDay,
  onTodo,
  saving,
  onRetime,
}: TimelineProps) {
  const scroll = useRef<HTMLDivElement>(null);
  const now = useNow();
  const [selected, setSelected] = useState<string | null>(null);
  useEffect(() => {
    if (!scroll.current) return;
    if (orientation === "horizontal") scroll.current.scrollLeft = 8 * 60;
    else scroll.current.scrollTop = 8 * 60;
    setSelected(null);
  }, [mode, orientation, days[0]]);
  const week = mode === "week";
  const drag = useTimelineDrag(orientation === "horizontal", saving, onRetime, onTodo);
  if (orientation === "horizontal") {
    return (
      <HorizontalTimeline
        days={days}
        items={items}
        week={week}
        trackCount={trackCount}
        now={now}
        selected={selected}
        onSelect={setSelected}
        saving={saving}
        onRetime={onRetime}
        drag={drag}
        onDay={onDay}
        onTodo={onTodo}
        scrollRef={scroll}
      />
    );
  }
  return (
    <div
      className={`calendar-timeline ${week ? "is-week" : "is-single-day"}`}
      ref={scroll}
      aria-busy={saving}
    >
      {drag.label && <div className="timeline-drag-label" role="status">{drag.label}</div>}
      <div className="timeline-inner" style={{ "--day-count": days.length }}>
        <div className="timeline-head">
          <span>24h</span>
          {days.map((key) => (
            <button
              type="button"
              key={key}
              onClick={() => onDay(key)}
              aria-label={`查看${key}`}
              className={key === now.date ? "is-today" : ""}
            >
              {WEEKDAYS[(dateFromKey(key).getUTCDay() + 6) % 7]}{" "}
              <strong>{Number(key.slice(-2))}</strong>
            </button>
          ))}
        </div>
        <div className="timeline-body">
          <div className="timeline-hours">
            {Array.from({ length: 25 }, (_, hour) => (
              <span key={hour} style={{ top: hour * 60 }}>
                {String(hour).padStart(2, "0")}:00
              </span>
            ))}
          </div>
          {days.map((key) => {
            const { segments, tracks } = layoutDay(
              items,
              key,
              trackCount,
            );
            const todos = layoutTimelineTodos(items, key, tracks);
            return (
              <div className="timeline-day" key={key} aria-label={key}>
                {segments.map((segment) => {
                  const { item, track } = segment;
                  const delta = drag.preview?.item.id === item.id ? drag.preview.delta : 0;
                  const start = Math.max(0, Math.min(1439, segment.start + (drag.preview?.edge === "start" ? delta : 0)));
                  const end = Math.max(start + 1, Math.min(1440, segment.end + (drag.preview?.edge === "end" ? delta : 0)));
                  const columns = tracks;
                  const label = `${item.title} · ${item.date} ${item.time} — ${item.endDate} ${item.endTime}`;
                  return (
                    <button
                      type="button"
                      key={item.id}
                      className={`timeline-event ${selected === item.id ? "is-selected" : ""}`}
                      data-search-key={`event-${item.eventId}`}
                      data-track={track}
                      title={label}
                      aria-label={label}
                      {...drag.interaction(item)}
                      style={{
                        top: start,
                        height: Math.max(3, end - start),
                        left: `calc(${(track * 100) / columns}% + 2px)`,
                        width: `max(3px, calc(${100 / columns}% - 4px))`,
                        "--event-color": item.color,
                        "--event-soft": colorWithAlpha(item.color, 0.18),
                      }}
                    >
                      {!segment.continuesBefore && <span className="timeline-resize is-start" data-time-edge="start" title="拖动调整开始时间" />}
                      {!segment.continuesAfter && <span className="timeline-resize is-end" data-time-edge="end" title="拖动调整结束时间" />}
                      <span className="timeline-event-title">
                        {segment.continuesBefore ? "↑ " : ""}
                        {item.title}
                        {segment.continuesAfter ? " ↓" : ""}
                      </span>
                    </button>
                  );
                })}
                {todos.map(({item, track, minute}) => (
                  <button type="button" key={item.id}
                    className={`timeline-todo ${item.occurrenceDone || item.allDone ? "is-done" : ""}`}
                    data-search-key={`todo-${item.todoId}`} data-track={track}
                    title={`${item.time} ${item.title}`} aria-label={`${item.time} ${item.title}`}
                    {...drag.interaction(item)}
                    style={{top: Math.max(0, Math.min(1439, minute + (drag.preview?.item.id === item.id ? drag.preview.delta : 0))),
                      left: `calc(${track * 100 / tracks}% + 2px)`, width: `calc(${100 / tracks}% - 4px)`,
                      "--event-color": item.color, zIndex: 10}}>
                    <span>{item.time} {item.title}</span>
                  </button>
                ))}
                {key === now.date && (
                  <div
                    className="timeline-now"
                    style={{
                      top:
                        Number(now.time.slice(0, 2)) * 60 +
                        Number(now.time.slice(3)),
                    }}
                    aria-label={`现在 ${now.time}`}
                  />
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function HorizontalTimeline({
  days,
  items,
  week,
  trackCount,
  now,
  selected,
  onDay,
  scrollRef,
  saving,
  drag,
}: Omit<TimelineProps, "mode" | "orientation"> & { drag: ReturnType<typeof useTimelineDrag>; week: boolean; now: ReturnType<typeof useNow>; selected: string | null; onSelect: (id: string | null) => void; scrollRef: React.RefObject<HTMLDivElement | null> }) {
  return (
    <div
      className={`calendar-timeline is-horizontal ${week ? "is-week" : "is-single-day"}`}
      ref={scrollRef}
      aria-busy={saving}
    >
      {drag.label && <div className="timeline-drag-label" role="status">{drag.label}</div>}
      <div className="horizontal-timeline-inner">
        <div className="horizontal-timeline-head">
          <span>24h</span>
          {Array.from({ length: 25 }, (_, hour) => (
            <time key={hour} style={{ left: 72 + hour * 60 }}>
              {String(hour).padStart(2, "0")}:00
            </time>
          ))}
        </div>
        {days.map((key) => {
          const { segments, tracks } = layoutDay(
            items,
            key,
            trackCount,
          );
          const todos = layoutTimelineTodos(items, key, tracks);
          const rowHeight = Math.max(week ? 78 : 180, tracks * 24 + 8);
          return (
            <div
              className="horizontal-timeline-day"
              key={key}
              style={{ "--horizontal-row-height": `${rowHeight}px` }}
            >
              <button
                type="button"
                className={key === now.date ? "is-today" : ""}
                onClick={() => onDay(key)}
                aria-label={`查看${key}`}
              >
                {WEEKDAYS[(dateFromKey(key).getUTCDay() + 6) % 7]}
                <strong>{Number(key.slice(-2))}</strong>
              </button>
              <div className="horizontal-timeline-track">
                {segments.map((segment) => {
                  const { item, track } = segment;
                  const delta = drag.preview?.item.id === item.id ? drag.preview.delta : 0;
                  const start = Math.max(0, Math.min(1439, segment.start + (drag.preview?.edge === "start" ? delta : 0)));
                  const end = Math.max(start + 1, Math.min(1440, segment.end + (drag.preview?.edge === "end" ? delta : 0)));
                  const label = `${item.title} · ${item.date} ${item.time} — ${item.endDate} ${item.endTime}`;
                  const eventAreaHeight = rowHeight - 4;
                  const laneHeight = eventAreaHeight / tracks;
                  return (
                    <button
                      type="button"
                      key={item.id}
                      className={`horizontal-event ${selected === item.id ? "is-selected" : ""}`}
                      data-search-key={`event-${item.eventId}`}
                      data-track={track}
                      title={label}
                      aria-label={label}
                      {...drag.interaction(item)}
                      style={{
                        left: start,
                        width: Math.max(3, end - start),
                        top: 2 + track * laneHeight,
                        height: Math.max(3, laneHeight - 4),
                        "--event-color": item.color,
                        "--event-soft": colorWithAlpha(item.color, 0.18),
                      }}
                    >
                      {!segment.continuesBefore && <span className="timeline-resize is-start" data-time-edge="start" title="拖动调整开始时间" />}
                      {!segment.continuesAfter && <span className="timeline-resize is-end" data-time-edge="end" title="拖动调整结束时间" />}
                      {item.title}
                    </button>
                  );
                })}
                {todos.map(({item, track, minute}) => (
                  <button type="button" key={item.id}
                    className={`horizontal-todo ${item.occurrenceDone || item.allDone ? "is-done" : ""}`}
                    data-search-key={`todo-${item.todoId}`} data-track={track}
                    title={`${item.time} ${item.title}`} aria-label={`${item.time} ${item.title}`}
                    {...drag.interaction(item)}
                    style={{left: Math.max(0, Math.min(1439, minute + (drag.preview?.item.id === item.id ? drag.preview.delta : 0))),
                      top: 2 + track * (rowHeight - 4) / tracks, height: (rowHeight - 4) / tracks - 4,
                      "--event-color": item.color, zIndex: 10}}>
                    <span>{item.time} {item.title}</span>
                  </button>
                ))}
                {key === now.date && (
                  <div
                    className="horizontal-timeline-now"
                    style={{
                      left:
                        Number(now.time.slice(0, 2)) * 60 +
                        Number(now.time.slice(3)),
                    }}
                    aria-label={`现在 ${now.time}`}
                  />
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
