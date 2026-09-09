import { colorWithAlpha, isEventDone } from "../lib/calendar";

export default function EventDot({ item, selected = false, onSelect }) {
	const done = isEventDone(item);
	const calendarEvent = item.kind === "event";

  return (
    <button
      className={`event-dot-button ${done ? "is-complete" : "is-pending"}`}
      type="button"
      style={{
        "--event-color": item.color,
        "--event-soft": colorWithAlpha(item.color, 0.17),
      }}
		aria-label={calendarEvent
			? `${item.title}，${item.time}至${item.endTime}`
			: `${item.title}，${item.time}，${done ? "已完成" : "待完成"}`}
      aria-pressed={selected}
      onClick={(event) => {
        event.stopPropagation();
			onSelect?.(item);
      }}
    >
      <span className="event-dot" aria-hidden="true" />
      <span className="event-preview" role="tooltip">
			<b>{calendarEvent ? `${item.time}–${item.endTime}` : item.time}</b>
        <span>{item.title}</span>
      </span>
    </button>
  );
}
