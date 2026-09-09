import { buildMonthDays, WEEKDAYS, TODAY_KEY } from "../lib/calendar";

export default function YearCalendar({ year, onMonth }) {
  return (
    <div className="year-calendar" aria-label={`${year}年`}>
      {Array.from({ length: 12 }, (_, month) => (
        <button
          type="button"
          className="year-month"
          key={month}
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
