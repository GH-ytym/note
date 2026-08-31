import { ArrowSquareOut, CalendarBlank } from "@phosphor-icons/react";
import { TODAY_KEY } from "../lib/calendar";

export default function LinkedDateField({ repeat, date, customDates, color, active, onOpen, onNativeChange }) {
  const custom = repeat === "自定义";
  const summary = custom
    ? customDates.length > 0 ? `已选 ${customDates.length} 天` : "尚未选择"
    : date;

  return (
    <div className="linked-date-field" style={{ "--linked-date-color": color || "#F3B51B" }}>
      <span>{custom ? "日期" : "开始日期"}</span>
      {window.noteDesktop?.isDesktop ? (
        <button type="button" onClick={onOpen} aria-label="在右侧日历中选择日期">
          <CalendarBlank size={17} />
          <strong>{summary}</strong>
          <small>{active ? "正在右侧日历选择" : "去右侧日历选择"}</small>
          <ArrowSquareOut size={16} />
        </button>
      ) : (
        <input type="date" min={TODAY_KEY} value={date} onChange={(event) => onNativeChange(event.target.value)} />
      )}
    </div>
  );
}
