import {
  CaretLeft,
  CaretRight,
  ArrowsIn,
  Crosshair,
  GearSix,
  Plus,
} from "@phosphor-icons/react";
import WindowControls from "./WindowControls";
import useWindowDrag from "../hooks/useWindowDrag";
import type { View, DayStyle } from "../types";
import type { PickerState } from "../desktop";
interface Props {
  search: React.ReactNode;
  title: string; view: View; onView: (view: View) => void; onMove: (amount: number) => void;
  onToday: () => void; onCreate: () => void; onSettings: () => void;
  picker: PickerState | null; onFinish: () => void; dayStyle: DayStyle;
  onDayStyle: (style: DayStyle) => void; onMini: () => void;
}

export default function CalendarToolbar({
  search,
  title,
  view,
  onView,
  onMove,
  onToday,
  onCreate,
  onSettings,
  picker,
  dayStyle,
  onDayStyle,
  onMini,
}: Props) {
  const drag = useWindowDrag();
  return (
    <header className="calendar-header" {...drag}>
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
        <span className="picker-instruction">{picker.field === "endDate" ? "选择结束日期" : "选择开始前日期"}</span>
      ) : (
        <>
          <div
            className="calendar-view-switch"
            role="group"
            aria-label="日历视图"
          >
            {([
              ["year", "年"],
              ["month", "月"],
              ["week", "周"],
              ["day", "日"],
            ] as const).map(([value, label]) => (
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
          {search}
          {view === "day" && (
            <div className="day-view-options">
              {([["timeline", "时间轴"], ["clock", "时钟"], ["tags", "标签"]] as const).map(([mode, label]) => (
                <label className="day-style-toggle" key={mode}>
                  <input type="checkbox" checked={dayStyle === mode} onChange={() => onDayStyle(mode)} />{label}
                </label>
              ))}
            </div>
          )}
          <div className="calendar-actions">
            <button onClick={onMini} title="小窗模式" aria-label="小窗模式">
              <ArrowsIn size={18} />
            </button>
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
      <WindowControls variant={picker ? "collapse" : "hide-app"} />
    </header>
  );
}
