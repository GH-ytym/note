import { useContext, useEffect, useRef } from "react";
import { InboxContext } from "./Workspace";
import {
  CaretLeft,
  CaretRight,
  ArrowsIn,
  Crosshair,
  Plus,
  Check,
  X,
} from "@phosphor-icons/react";
import WindowControls from "./WindowControls";
import useWindowDrag from "../hooks/useWindowDrag";
import type { View, DayStyle } from "../types";
import type { PickerState } from "../desktop";
interface Props {
  search: React.ReactNode;
  title: string; view: View; onView: (view: View) => void; onMove: (amount: number) => void;
  onToday: () => void; onCreate: () => void;
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
  picker,
  dayStyle,
  onDayStyle,
  onMini,
  onFinish,
}: Props) {
  const cancel = useRef<HTMLButtonElement>(null);
  useEffect(() => { if (picker) cancel.current?.focus(); }, [!!picker]);
  const drag = useWindowDrag();
  const inWorkspace = useContext(InboxContext) !== null;
  return (
    <header className="calendar-header" {...(inWorkspace ? {} : drag)}>
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
        <div className="picker-actions"><span className="picker-instruction">{picker.field === "endDate" ? "选择结束日期" : "选择开始日期"}</span><button ref={cancel} type="button" aria-label="取消日期选择" title="取消日期选择" onClick={onFinish}><X size={18} /></button><button className="picker-confirm" type="button" aria-label="完成日期选择" title="完成日期选择" onClick={onFinish}><Check size={18} /></button></div>
      ) : (
        <>
          <div
            className="calendar-view-switch"
            role="group"
            aria-label="日历视图"
          >
            <span className="view-slider" aria-hidden="true" style={{ transform: `translateX(${["year", "month", "week", "day"].indexOf(view) * 30}px)` }} />
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
              <span className="view-slider" aria-hidden="true" style={{ transform: `translateX(${["year", "month", "week", "day"].indexOf(view) * 30}px)` }} />
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
          </div>
        </>
      )}
      {!inWorkspace && <WindowControls variant={picker ? "collapse" : "hide-app"} />}
    </header>
  );
}
