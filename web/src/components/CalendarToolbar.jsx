import {
  CaretLeft,
  CaretRight,
  Check,
  Crosshair,
  GearSix,
  Plus,
} from "@phosphor-icons/react";
import WindowControls from "./WindowControls";

export default function CalendarToolbar({
  title,
  view,
  onView,
  onMove,
  onToday,
  onCreate,
  onSettings,
  picker,
  onFinish,
  dayStyle,
  onDayStyle,
}) {
  return (
    <header className="calendar-header">
      <span className="title-accent" aria-hidden="true" />
      <h1>{title}</h1>
      <div className="month-controls">
        <button
          type="button"
          onClick={() => onMove(-1)}
          aria-label="上一段日期"
        >
          <CaretLeft size={17} />
        </button>
        <button type="button" onClick={() => onMove(1)} aria-label="下一段日期">
          <CaretRight size={17} />
        </button>
      </div>
      {picker ? (
        <button
          className="picker-finish"
          onClick={onFinish}
          aria-label="完成日期选择"
        >
          <Check size={17} />
        </button>
      ) : (
        <>
          <div
            className="calendar-view-switch"
            role="group"
            aria-label="日历视图"
          >
            {[
              ["year", "年"],
              ["month", "月"],
              ["week", "周"],
              ["day", "日"],
            ].map(([value, label]) => (
              <button
                type="button"
                key={value}
                aria-pressed={view === value}
                onClick={() => onView(value)}
              >
                {label}
              </button>
            ))}
          </div>
          {view === "day" && (
            <label className="day-style-toggle">
              <input
                type="checkbox"
                checked={dayStyle === "clock"}
                onChange={(e) =>
                  onDayStyle(e.target.checked ? "clock" : "timeline")
                }
              />
              时钟
            </label>
          )}
          <div className="calendar-actions">
            <button onClick={onToday} title="回到今天" aria-label="回到今天">
              <Crosshair size={18} />
            </button>
            <button onClick={onCreate} title="新建" aria-label="新建">
              <Plus size={18} />
            </button>
            <button onClick={onSettings} title="设置" aria-label="设置">
              <GearSix size={18} />
            </button>
          </div>
        </>
      )}
      <WindowControls variant="hide-app" />
    </header>
  );
}
