import { ArrowSquareOut, CalendarBlank } from "@phosphor-icons/react";
import { TODAY_KEY } from "../lib/calendar";

export default function LinkedDateField({ repeat, date, customDates = [], color, active, onOpen, onNativeChange, label, min = TODAY_KEY }) {
  const custom = repeat === "自定义";
  const summary = custom
    ? customDates.length > 0 ? `已选 ${customDates.length} 天` : "尚未选择"
    : date;

  return (
    <div className="linked-date-field" style={{ "--linked-date-color": color || "#F3B51B" }}>
      <span>{label || (custom ? "日期" : "开始日期")}</span>
      {window.noteDesktop?.isDesktop ? (
        <button type="button" onClick={onOpen} aria-label={`选择${label || "开始日期"}`}>
          <CalendarBlank size={17} />
          <strong>{summary}</strong>
          <ArrowSquareOut size={16} />
        </button>
      ) : (
        <input type="date" min={min} value={date} onChange={(event) => onNativeChange(event.target.value)} />
      )}
    </div>
  );
}
