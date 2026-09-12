import { useEffect, useRef } from "react";
import { buildMonthDays, WEEKDAYS, TODAY_KEY } from "../lib/calendar";

export default function YearCalendar({ year, onMonth, focusMonth, focusPulse }: { year: number; onMonth: (month: Date) => void; focusMonth: number; focusPulse: number }) {
  const focusTarget = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!focusPulse) return;
    const reducedMotion = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    focusTarget.current?.scrollIntoView({
      behavior: reducedMotion ? "auto" : "smooth",
      block: "center",
    });
  }, [focusPulse, year]);

  return (
    <div className="year-calendar" aria-label={`${year}年`}>
      {Array.from({ length: 12 }, (_, month) => (
        <button
          type="button"
          ref={month === focusMonth ? focusTarget : undefined}
          className={`year-month ${focusPulse && month === focusMonth ? "is-today-pulse" : ""}`}
          key={`${month}-${month === focusMonth ? focusPulse : 0}`}
          aria-label={`${year}年${month + 1}月`}
          onClick={() => onMonth(new Date(Date.UTC(year, month, 1)))}
        >
          <strong>{month + 1}月</strong>
          <span className="mini-month-grid">
            {WEEKDAYS.map((day) => (
              <span className="mini-weekday" key={day}>
                {day}
              </span>
            ))}
            {buildMonthDays(new Date(Date.UTC(year, month, 1))).map((day) => (
              <span
                key={day.key}
                className={day.key === TODAY_KEY ? "mini-today" : ""}
              >
                {day.isCurrentMonth ? day.day : ""}
              </span>
            ))}
          </span>
        </button>
      ))}
    </div>
  );
}
