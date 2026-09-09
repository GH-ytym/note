import { useRef, useState } from "react";
import {
  clockArc,
  clockMinute,
  clockPoint,
  clockTrackRadius,
  minuteDelta,
  stableClockTrack,
} from "../lib/clock";
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
}) {
  const svg = useRef(null),
    drag = useRef(null),
    suppressClick = useRef(false);
  const [preview, setPreview] = useState(null),
    [hover, setHover] = useState(null);
  const { segments } = layoutDay(items, date);
  const minuteAt = (event) => {
    const rect = svg.current.getBoundingClientRect();
    return clockMinute(
      ((event.clientX - rect.left) * 360) / rect.width,
      ((event.clientY - rect.top) * 360) / rect.height,
    );
  };
  function begin(event, item, edge) {
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
  function move(event) {
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
    setPreview(null);
    suppressClick.current = Boolean(active?.moved);
    if (!cancelled && active?.moved) {
      const delta = Math.round(active.total / 5) * 5;
      if (delta) void onRetime(active.item, active.edge, delta);
    }
  }
  function open(item) {
    if (!suppressClick.current) onOpen(item);
    suppressClick.current = false;
  }
  function keys(event, item, edge) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen(item);
    }
    if (["ArrowLeft", "ArrowRight"].includes(event.key) && !saving) {
      event.preventDefault();
      void onRetime(item, edge, event.key === "ArrowLeft" ? -5 : 5);
    }
  }
  const interaction = (item, edge) => ({
    onPointerDown: (e) => begin(e, item, edge),
    onClick: () => open(item),
    onKeyDown: (e) => keys(e, item, edge),
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
        viewBox="0 0 360 360"
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
          ({ item, start, end, continuesBefore, continuesAfter }) => {
            const delta = preview?.id === item.id ? preview.delta : 0;
            const a = start + (preview?.edge === "start" ? delta : 0),
              b = end + (preview?.edge === "end" ? delta : 0);
            const track = stableClockTrack(item, trackCount);
            const radius = clockTrackRadius(track, trackCount);
            const head = clockPoint(a, radius),
              tail = clockPoint(b, radius);
            return (
              <g key={item.id} style={{ "--clock-item": item.color }}>
                <path
                  d={clockArc(a, Math.max(a + 1, b), radius)}
                  className="clock-event"
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
        {items
          .filter((item) => item.kind === "todo" && item.date === date)
          .map((item, index, all) => {
            const delta = preview?.id === item.id ? preview.delta : 0;
            const minute =
              Number(item.time.slice(0, 2)) * 60 +
              Number(item.time.slice(3)) +
              delta;
            const peers = all.filter((other) => other.time === item.time),
              lane = peers.findIndex((other) => other.id === item.id);
            const a = clockPoint(minute, 151 + lane * 5),
              b = clockPoint(minute, 166 + lane * 5);
            return (
              <g
                key={item.id}
                className={`clock-todo ${item.allDone || item.occurrenceDone ? "is-done" : ""}`}
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
          ? `${preview.delta > 0 ? "+" : ""}${preview.delta} 分钟`
          : hover
            ? `${hover.time} ${hover.title}`
            : "\u00a0"}
      </div>
    </div>
  );
}
