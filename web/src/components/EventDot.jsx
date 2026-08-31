import { colorWithAlpha, isEventDone } from "../lib/calendar";

export default function EventDot({ item, selected = false, onSelect }) {
  const done = isEventDone(item);

  return (
    <button
      className={`event-dot-button ${done ? "is-complete" : "is-pending"}`}
      type="button"
      style={{
        "--event-color": item.color,
        "--event-soft": colorWithAlpha(item.color, 0.17),
      }}
      aria-label={`${item.title}，${item.time}，${done ? "已完成" : "待完成"}`}
      aria-pressed={selected}
      onClick={(event) => {
        event.stopPropagation();
        onSelect(item);
      }}
    >
      <span className="event-dot" aria-hidden="true" />
      <span className="event-preview" role="tooltip">
        <b>{item.time}</b>
        <span>{item.title}</span>
      </span>
    </button>
  );
}
