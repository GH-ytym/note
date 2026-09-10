import { useEffect, useRef, useState } from "react";
import {
  WEEKDAYS,
  dateFromKey,
  colorWithAlpha,
  shanghaiDateTimeParts,
} from "../lib/calendar";
import { layoutDay } from "../lib/timeline";
import useNow from "../hooks/useNow";

export default function CalendarTimeline({
  days,
  items,
  mode,
  orientation = "vertical",
  trackCount = 7,
  onDay,
  onTodo,
}) {
  const scroll = useRef(null);
  const now = useNow();
  const [selected, setSelected] = useState(null);
  useEffect(() => {
    if (orientation === "horizontal") scroll.current.scrollLeft = 8 * 60;
    else scroll.current.scrollTop = 8 * 60;
    setSelected(null);
  }, [mode, orientation, days[0]]);
  const week = mode === "week";
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
    >
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
            const todos = items.filter(
              (item) => item.kind === "todo" && item.date === key,
            );
            return (
              <div className="timeline-day" key={key} aria-label={key}>
                {segments.map((segment) => {
                  const { item, start, end, track } = segment;
                  const columns = tracks;
                  const label = `${item.title} · ${item.date} ${item.time} — ${item.endDate} ${item.endTime}`;
                  return (
                    <button
                      type="button"
                      key={item.id}
                      className={`timeline-event ${selected === item.id ? "is-selected" : ""}`}
                      title={label}
                      aria-label={label}
                      onClick={() => onTodo(item)}
                      style={{
                        top: start,
                        height: Math.max(3, end - start),
                        left: `calc(${(track * 100) / columns}% + 2px)`,
                        width: `max(3px, calc(${100 / columns}% - 4px))`,
                        "--event-color": item.color,
                        "--event-soft": colorWithAlpha(item.color, 0.18),
                      }}
                    >
                      <span className="timeline-event-title">
                        {segment.continuesBefore ? "↑ " : ""}
                        {item.title}
                        {segment.continuesAfter ? " ↓" : ""}
                      </span>
                    </button>
                  );
                })}
                {todos.map((item, index) => {
                  const [hour, minute] = item.time.split(":").map(Number);
                  const peers = todos.filter(
                    (other) => other.time === item.time,
                  );
                  const peer = peers.findIndex((other) => other.id === item.id);
                  return (
                    <button
                      type="button"
                      className={`timeline-todo ${item.occurrenceDone || item.allDone ? "is-done" : ""}`}
                      key={item.id}
                      title={`${item.time} ${item.title}`}
                      aria-label={`${item.time} ${item.title}`}
                      onClick={() => onTodo(item)}
                      style={{
                        top: hour * 60 + minute,
                        "--event-color": item.color,
                        zIndex: 5 + index,
                      }}
                    >
                      <span
                        style={{
                          left: `${(peer * 100) / peers.length}%`,
                          maxWidth: `${100 / peers.length}%`,
                        }}
                      >
                        {item.time} {item.title}
                      </span>
                    </button>
                  );
                })}
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
  onSelect,
  onDay,
  onTodo,
  scrollRef,
}) {
  return (
    <div
      className={`calendar-timeline is-horizontal ${week ? "is-week" : "is-single-day"}`}
      ref={scrollRef}
    >
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
          const todos = items.filter(
            (item) => item.kind === "todo" && item.date === key,
          );
          const rowHeight = week ? 78 : Math.max(180, tracks * 24 + 44);
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
                  const { item, start, end, track } = segment;
                  const label = `${item.title} · ${item.date} ${item.time} — ${item.endDate} ${item.endTime}`;
                  const eventAreaHeight = rowHeight - 30;
                  const laneHeight = eventAreaHeight / tracks;
                  return (
                    <button
                      type="button"
                      key={item.id}
                      className={`horizontal-event ${selected === item.id ? "is-selected" : ""}`}
                      title={label}
                      aria-label={label}
                      onClick={() => onTodo(item)}
                      style={{
                        left: start,
                        width: Math.max(3, end - start),
                        top: 2 + track * laneHeight,
                        height: Math.max(3, laneHeight - 4),
                        "--event-color": item.color,
                        "--event-soft": colorWithAlpha(item.color, 0.18),
                      }}
                    >
                      {item.title}
                    </button>
                  );
                })}
                {todos.map((item, index) => {
                  const [hour, minute] = item.time.split(":").map(Number);
                  return (
                    <button
                      type="button"
                      className={`horizontal-todo ${item.occurrenceDone || item.allDone ? "is-done" : ""}`}
                      key={item.id}
                      title={`${item.time} ${item.title}`}
                      aria-label={`${item.time} ${item.title}`}
                      onClick={() => onTodo(item)}
                      style={{
                        left: hour * 60 + minute,
                        top: rowHeight - 26 - (index % 2) * 13,
                        "--event-color": item.color,
                      }}
                    >
                      <span>{item.title}</span>
                    </button>
                  );
                })}
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
