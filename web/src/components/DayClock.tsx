import type { CalendarItem } from "../types";
type Edge = "start" | "end";
import { useEffect, useRef, useState } from "react";
import {
  clockArc,
  clockMinute,
  clockPoint,
  clockTrackRadius,
  layoutTodoTracks,
  minuteDelta,
} from "../lib/clock";
import { dragTimeLabel } from "../lib/drag-time";
import { layoutDay } from "../lib/timeline";

export default function DayClock({
  date,
  items,
  now,
  handMode,
  trackCount,
  onOpen,
  onRetime,
  saving,
  mini = false,
}: { date: string; items: CalendarItem[]; now: { date: string; time: string; second: number; millisecond: number }; handMode: string; trackCount: number; onOpen: (item: CalendarItem) => void; onRetime: (item: CalendarItem, edge: Edge, minutes: number) => Promise<void>; saving: boolean; mini?: boolean }) {
  const svg = useRef<SVGSVGElement>(null),
    drag = useRef<{ item: CalendarItem; edge: Edge; last: number; total: number; x: number; y: number; moved: boolean } | null>(null),
    suppressClick = useRef(false);
  const [preview, setPreview] = useState<{ id: string; edge: Edge; delta: number } | null>(null),
    [hover, setHover] = useState<CalendarItem | null>(null);
  const { segments } = layoutDay(items, date, trackCount);
  const todos = layoutTodoTracks(items, date, trackCount);
  const outerTrack = Math.max(0, ...todos.map(todo => todo.track));
  const extent = Math.max(180, 176 + outerTrack * 20);
  useEffect(() => {
    if (mini) void window.noteDesktop?.fitMiniWindow?.({ size: extent * 2 });
  }, [mini, extent]);
  const minuteAt = (event: React.PointerEvent) => {
    const rect = svg.current!.getBoundingClientRect();
    return clockMinute(
      180 - extent + ((event.clientX - rect.left) * extent * 2) / rect.width,
      180 - extent + ((event.clientY - rect.top) * extent * 2) / rect.height,
    );
  };
  function begin(event: React.PointerEvent, item: CalendarItem, edge: Edge) {
    if (saving || event.button !== 0) return;
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = {
      item,
      edge,
      last: minuteAt(event),
      total: 0,
      x: event.clientX,
      y: event.clientY,
      moved: false,
    };
    suppressClick.current = false;
  }
  function move(event: React.PointerEvent) {
    const active = drag.current;
    if (!active) return;
    const minute = minuteAt(event);
    active.total += minuteDelta(active.last, minute);
    active.last = minute;
    if (Math.hypot(event.clientX - active.x, event.clientY - active.y) > 5)
      active.moved = true;
    if (active.moved)
      setPreview({
        id: active.item.id,
        edge: active.edge,
        delta: Math.round(active.total / 5) * 5,
      });
  }
  function finish(cancelled = false) {
    const active = drag.current;
    drag.current = null;
    if (!active) return;
    setPreview(null);
    suppressClick.current = Boolean(active?.moved);
    if (!cancelled && active?.moved) {
      const delta = Math.round(active.total / 5) * 5;
      if (delta) void onRetime(active.item, active.edge, delta);
    }
  }
  function open(item: CalendarItem) {
    if (!suppressClick.current) onOpen(item);
    suppressClick.current = false;
  }
  function keys(event: React.KeyboardEvent, item: CalendarItem, edge: Edge) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen(item);
    }
    if (["ArrowLeft", "ArrowRight"].includes(event.key) && !saving) {
      event.preventDefault();
      void onRetime(item, edge, event.key === "ArrowLeft" ? -5 : 5);
    }
  }
  const interaction = (item: CalendarItem, edge: Edge) => ({
    onPointerDown: (e: React.PointerEvent) => begin(e, item, edge),
    onClick: () => open(item),
    onKeyDown: (e: React.KeyboardEvent) => keys(e, item, edge),
    onPointerEnter: () => setHover(item),
    onPointerLeave: () => setHover(null),
    role: "button",
    tabIndex: 0,
  });
  const hour = Number(now.time.slice(0, 2));
  const minute = Number(now.time.slice(3));
  const second = now.second + now.millisecond / 1000;
  const nowMinute = hour * 60 + minute + second / 60;
  const compactHand = clockPoint(nowMinute, 105);
  const hourHand = clockPoint(nowMinute, 73);
  const minuteHand = clockPoint((minute + second / 60) * 24, 100);
  const secondHand = clockPoint(second * 24, 108);
  const isToday = date === now.date;
  return (
    <div className="day-clock-wrap" aria-busy={saving}>
      <svg
        ref={svg}
        className="day-clock"
        viewBox={`${180 - extent} ${180 - extent} ${extent * 2} ${extent * 2}`}
        aria-label={`${date} 24小时日程钟`}
        onPointerMove={move}
        onPointerUp={() => finish()}
        onPointerCancel={() => finish(true)}
        onLostPointerCapture={() => {
          if (drag.current) finish(true);
        }}
      >
        <circle cx="180" cy="180" r="119" className="clock-face" />
        {Array.from({ length: 96 }, (_, index) => {
          const a = clockPoint(index * 15, 138),
            b = clockPoint(index * 15, index % 4 === 0 ? 128 : 134);
          return (
            <line
              key={index}
              x1={a.x}
              y1={a.y}
              x2={b.x}
              y2={b.y}
              className={index % 4 === 0 ? "clock-hour-tick" : "clock-tick"}
            />
          );
        })}
        {Array.from({ length: 12 }, (_, index) => {
          const p = clockPoint(index * 120, 151);
          return (
            <text x={p.x} y={p.y} key={index} className="clock-hour-label">
              {index * 2}
            </text>
          );
        })}
        {segments.map(
          ({ item, start, end, track, continuesBefore, continuesAfter }) => {
            const delta = preview?.id === item.id ? preview.delta : 0;
            const a = start + (preview?.edge === "start" ? delta : 0),
              b = end + (preview?.edge === "end" ? delta : 0);
            const radius = clockTrackRadius(track);
            const head = clockPoint(a, radius),
              tail = clockPoint(b, radius);
            return (
              <g key={item.id} style={{ "--clock-item": item.color }}>
                <path
                  d={clockArc(a, Math.max(a + 1, b), radius)}
                  className="clock-event"
                  data-search-key={`event-${item.eventId}`}
                  {...interaction(item, "start")}
                  aria-label={`${item.title} ${item.time} 至 ${item.endTime}`}
                >
                  <title>{`${item.title}\n${item.date} ${item.time} — ${item.endDate} ${item.endTime}`}</title>
                </path>
                {!continuesBefore && (
                  <circle
                    cx={head.x}
                    cy={head.y}
                    r="5"
                    className="clock-handle"
                    {...interaction(item, "start")}
                    aria-label={`调整${item.title}开始时间`}
                  />
                )}
                {!continuesAfter && (
                  <circle
                    cx={tail.x}
                    cy={tail.y}
                    r="5"
                    className="clock-handle"
                    {...interaction(item, "end")}
                    aria-label={`调整${item.title}结束时间`}
                  />
                )}
              </g>
            );
          },
        )}
        {todos.map(({ item, track }) => {
            const delta = preview?.id === item.id ? preview.delta : 0;
            const minute =
              Number(item.time.slice(0, 2)) * 60 +
              Number(item.time.slice(3)) +
              delta;
            const a = clockPoint(minute, 155 + track * 20),
              b = clockPoint(minute, 167 + track * 20);
            return (
              <g
                key={item.id}
                className={`clock-todo ${item.allDone || item.occurrenceDone ? "is-done" : ""}`}
                data-search-key={`todo-${item.todoId}`}
                style={{ "--clock-item": item.color }}
                {...interaction(item, "start")}
                aria-label={`${item.title} ${item.time}`}
              >
                <title>{`${item.time} ${item.title}`}</title>
                <line
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  className="clock-hit"
                />
                <line
                  x1={a.x}
                  y1={a.y}
                  x2={b.x}
                  y2={b.y}
                  className="clock-pin"
                />
                <circle cx={b.x} cy={b.y} r="2.5" />
              </g>
            );
          })}
        {isToday && (
          <g className={`clock-now is-${handMode}`}>
            {handMode === "full" ? (
              <>
                <line
                  className="clock-hour-hand"
                  x1="180"
                  y1="180"
                  x2={hourHand.x}
                  y2={hourHand.y}
                />
                <line
                  className="clock-minute-hand"
                  x1="180"
                  y1="180"
                  x2={minuteHand.x}
                  y2={minuteHand.y}
                />
                <line
                  className="clock-second-hand"
                  x1="180"
                  y1="180"
                  x2={secondHand.x}
                  y2={secondHand.y}
                />
              </>
            ) : (
              <line
                className="clock-compact-hand"
                x1="180"
                y1="180"
                x2={compactHand.x}
                y2={compactHand.y}
              />
            )}
            <circle cx="180" cy="180" r="3" />
            <text x="180" y="210" className="clock-digital">
              {handMode === "full"
                ? `${now.time}:${String(now.second).padStart(2, "0")}`
                : now.time}
            </text>
          </g>
        )}
      </svg>
      <div className="clock-selection" role="status">
        {preview
          ? `${drag.current?.item.title} · ${drag.current ? dragTimeLabel(drag.current.item, preview.edge, preview.delta) : ""}`
          : hover
            ? `${hover.time} ${hover.title}`
            : "\u00a0"}
      </div>
    </div>
  );
}
