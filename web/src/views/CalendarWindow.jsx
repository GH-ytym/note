import { useEffect, useMemo, useRef, useState } from "react";
import { CaretDown, CaretUp, Check } from "@phosphor-icons/react";
import { getCalendar } from "../api";
import CalendarGrid from "../components/CalendarGrid";
import WindowControls from "../components/WindowControls";
import {
  IS_DESKTOP,
  REPEAT_LABELS,
  TODAY_KEY,
  addMonths,
  buildGridOutlinePath,
  buildMonthDays,
  calendarEventFromOccurrence,
  calendarRange,
  colorWithAlpha,
  dateFromKey,
  isPastDateKey,
  monthFromKey,
  monthLabel,
  readableSelectionInk,
} from "../lib/calendar";

export default function CalendarWindow() {
  const [currentMonth, setCurrentMonth] = useState(() => monthFromKey(TODAY_KEY));
  const [events, setEvents] = useState([]);
  const [calendarLoading, setCalendarLoading] = useState(false);
  const [expandedDayKey, setExpandedDayKey] = useState(null);
  const [notice, setNotice] = useState("");
  const [desktopPicker, setDesktopPicker] = useState(null);
  const [hoveredStartDate, setHoveredStartDate] = useState(null);
  const dragSelection = useRef({ active: false, select: true, lastKey: null });
  const desktopPickerRef = useRef(null);
  const desktopPickerSessionRef = useRef(null);

  const monthDays = useMemo(() => buildMonthDays(currentMonth), [currentMonth]);
  const range = useMemo(() => calendarRange(currentMonth), [currentMonth]);
  const pickingRepeat = REPEAT_LABELS[desktopPicker?.repeatMode] || "仅一次";
  const pickingDate = desktopPicker?.date || TODAY_KEY;
  const pickingCustomDates = desktopPicker?.customDates || [];
  const customDateSet = useMemo(() => new Set(pickingCustomDates), [pickingCustomDates]);
  const customPicking = Boolean(desktopPicker) && pickingRepeat === "自定义";
  const startDatePicking = Boolean(desktopPicker) && pickingRepeat !== "自定义";
  const calendarPicking = customPicking || startDatePicking;

  function matchesStartPattern(day, anchorDate) {
    if (!anchorDate || isPastDateKey(anchorDate) || isPastDateKey(day.key)) return false;
    const active = dateFromKey(anchorDate);
    if (pickingRepeat === "每周") return dateFromKey(day.key).getUTCDay() === active.getUTCDay();
    if (pickingRepeat === "每月") return day.day === active.getUTCDate();
    return day.key === anchorDate;
  }

  function isHoveredStartPattern(day) {
    return startDatePicking && Boolean(hoveredStartDate) && matchesStartPattern(day, hoveredStartDate);
  }

  function isSelectedStartPattern(day) {
    return startDatePicking && Boolean(pickingDate) && matchesStartPattern(day, pickingDate);
  }

  const outlinedCellIndexes = monthDays.reduce((indexes, day, index) => {
    if (isPastDateKey(day.key)) return indexes;
    if ((customPicking && customDateSet.has(day.key)) || isHoveredStartPattern(day)) indexes.push(index);
    return indexes;
  }, []);
  const selectionOutlinePath = buildGridOutlinePath(outlinedCellIndexes);

  async function refreshCalendar() {
    const result = await getCalendar(range.from, range.to);
    const nextEvents = result.data.map(calendarEventFromOccurrence);
    setEvents(nextEvents);
    return nextEvents;
  }

  useEffect(() => {
    let cancelled = false;
    setCalendarLoading(true);
    getCalendar(range.from, range.to)
      .then((result) => {
        if (!cancelled) setEvents(result.data.map(calendarEventFromOccurrence));
      })
      .catch((error) => {
        if (!cancelled) setNotice(error.message);
      })
      .finally(() => {
        if (!cancelled) setCalendarLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [range.from, range.to]);

  useEffect(() => {
    if (!window.noteDesktop?.onDataChanged) return undefined;
    return window.noteDesktop.onDataChanged(() => {
      refreshCalendar().catch((error) => setNotice(error.message));
    });
  }, [range.from, range.to]);

  useEffect(() => {
    if (!window.noteDesktop?.getDatePickerState || !window.noteDesktop?.onDatePickerStateChanged) {
      return undefined;
    }

    let disposed = false;
    function applyPickerState(state) {
      if (disposed) return;
      desktopPickerRef.current = state;
      setDesktopPicker(state);
      setHoveredStartDate(null);

      if (state && desktopPickerSessionRef.current !== state.sessionId) {
        desktopPickerSessionRef.current = state.sessionId;
        setCurrentMonth(monthFromKey(state.date));
        setExpandedDayKey(null);
      }
      if (!state) desktopPickerSessionRef.current = null;
    }

    const removeListener = window.noteDesktop.onDatePickerStateChanged(applyPickerState);
    window.noteDesktop.getDatePickerState()
      .then(applyPickerState)
      .catch((error) => setNotice(error.message));

    return () => {
      disposed = true;
      removeListener?.();
    };
  }, []);

  useEffect(() => {
    function endDragSelection() {
      dragSelection.current.active = false;
      dragSelection.current.lastKey = null;
    }
    window.addEventListener("pointerup", endDragSelection);
    window.addEventListener("pointercancel", endDragSelection);
    return () => {
      window.removeEventListener("pointerup", endDragSelection);
      window.removeEventListener("pointercancel", endDragSelection);
    };
  }, []);

  useEffect(() => {
    if (!notice) return undefined;
    const timer = window.setTimeout(() => setNotice(""), 2400);
    return () => window.clearTimeout(timer);
  }, [notice]);

  function moveMonth(amount) {
    setCurrentMonth((month) => addMonths(month, amount));
    setExpandedDayKey(null);
  }

  function publishDesktopDateSelection(nextState) {
    desktopPickerRef.current = nextState;
    setDesktopPicker(nextState);
    const request = window.noteDesktop?.selectDatePicker?.({
      date: nextState.date,
      customDates: nextState.customDates,
    });
    request?.catch((error) => setNotice(error.message));
  }

  function updateCustomDate(dateKey, shouldSelect) {
    const activePicker = desktopPickerRef.current;
    if (!activePicker) return;
    const nextDates = new Set(activePicker.customDates || []);
    if (shouldSelect) nextDates.add(dateKey);
    else nextDates.delete(dateKey);
    const customDates = [...nextDates].sort();
    publishDesktopDateSelection({
      ...activePicker,
      date: customDates[0] || activePicker.date,
      customDates,
    });
  }

  function beginDateSelection(dateKey, event) {
    if (!calendarPicking || isPastDateKey(dateKey) || event.button !== 0) return;
    event.preventDefault();

    if (startDatePicking) {
      const activePicker = desktopPickerRef.current;
      if (activePicker) publishDesktopDateSelection({ ...activePicker, date: dateKey, customDates: [] });
      return;
    }

    const shouldSelect = !customDateSet.has(dateKey);
    dragSelection.current = { active: true, select: shouldSelect, lastKey: dateKey };
    updateCustomDate(dateKey, shouldSelect);
  }

  function continueDateSelection(dateKey, pastDateDisabled) {
    if (pastDateDisabled) {
      setHoveredStartDate(null);
      return;
    }
    if (startDatePicking) setHoveredStartDate(dateKey);
    const drag = dragSelection.current;
    if (!customPicking || !drag.active || drag.lastKey === dateKey) return;
    drag.lastKey = dateKey;
    updateCustomDate(dateKey, drag.select);
  }

  function toggleDateFromKeyboard(dateKey, event) {
    if (!calendarPicking || isPastDateKey(dateKey) || !["Enter", " "].includes(event.key)) return;
    event.preventDefault();
    if (startDatePicking) {
      const activePicker = desktopPickerRef.current;
      if (activePicker) publishDesktopDateSelection({ ...activePicker, date: dateKey, customDates: [] });
    } else {
      updateCustomDate(dateKey, !customDateSet.has(dateKey));
    }
  }

  function finishDesktopDatePicking() {
    if (!desktopPickerRef.current) return;
    const request = window.noteDesktop?.finishDatePicker?.();
    request?.catch((error) => setNotice(error.message));
  }

  function openDay(date) {
    const request = window.noteDesktop?.openDay?.({ date });
    request?.catch((error) => setNotice(error.message));
  }

  function openEvent(item) {
    const request = window.noteDesktop?.openDetail?.({ todoId: item.todoId, date: item.date });
    request?.catch((error) => setNotice(error.message));
  }

  const selectionColor = desktopPicker?.color || "#F3B51B";

  return (
    <main
      className={`calendar-shell ${IS_DESKTOP ? "is-desktop-calendar" : ""} ${calendarPicking ? "is-calendar-picking is-desktop-date-picking" : ""}`}
      style={{
        "--selection-color": selectionColor,
        "--selection-soft": colorWithAlpha(selectionColor, 0.12),
        "--selection-ink": readableSelectionInk(selectionColor),
      }}
    >
      <div className="workspace-stack">
        <div className="month-stack">
          <section
            className={`calendar-panel ${calendarLoading ? "is-loading" : ""}`}
            aria-label={`${monthLabel(currentMonth)}日历`}
            aria-busy={calendarLoading}
          >
            <header className="calendar-header">
              <span className="title-accent" aria-hidden="true" />
              <h1>{monthLabel(currentMonth)}</h1>
              <div className="month-controls">
                <button type="button" onClick={() => moveMonth(-1)} aria-label="切换到上个月">
                  <CaretUp size={19} weight="bold" />
                </button>
                <button type="button" onClick={() => moveMonth(1)} aria-label="切换到下个月">
                  <CaretDown size={19} weight="bold" />
                </button>
              </div>
              {desktopPicker && (
                <div className="desktop-picker-status" role="status">
                  <span className="desktop-picker-dot" aria-hidden="true" />
                  <div>
                    <strong>{pickingRepeat === "自定义" ? "选择自定义日期" : `选择${pickingRepeat}的开始日期`}</strong>
                    <small>{pickingRepeat === "自定义" ? `已选 ${pickingCustomDates.length} 天` : pickingDate}</small>
                  </div>
                  <button type="button" onClick={finishDesktopDatePicking}>
                    <Check size={15} weight="bold" />
                    返回填写
                  </button>
                </div>
              )}
              <WindowControls variant="collapse" />
            </header>

            <CalendarGrid
              days={monthDays}
              events={events}
              expandedDayKey={expandedDayKey}
              setExpandedDayKey={setExpandedDayKey}
              calendarPicking={calendarPicking}
              customPicking={customPicking}
              startDatePicking={startDatePicking}
              customDateSet={customDateSet}
              isPastDate={isPastDateKey}
              isHoveredStartPattern={isHoveredStartPattern}
              isSelectedStartPattern={isSelectedStartPattern}
              pickingRepeat={pickingRepeat}
              selectionOutlinePath={selectionOutlinePath}
              onBeginSelection={beginDateSelection}
              onContinueSelection={continueDateSelection}
              onKeyboardSelection={toggleDateFromKeyboard}
              onOpenDay={openDay}
              onOpenEvent={openEvent}
              onPointerLeave={() => setHoveredStartDate(null)}
            />
          </section>
        </div>
      </div>

      {notice && <div className="toast" role="status">{notice}</div>}
    </main>
  );
}
