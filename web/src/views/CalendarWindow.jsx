import { useEffect, useMemo, useRef, useState } from "react";
import CalendarToolbar from "../components/CalendarToolbar";
import DayClock from "../components/DayClock";
import useNow from "../hooks/useNow";
import useCalendarItems from "../hooks/useCalendarItems";
import { retimeItem } from "../lib/retime";
import CalendarGrid from "../components/CalendarGrid";
import YearCalendar from "../components/YearCalendar";
import CalendarTimeline from "../components/CalendarTimeline";
import {
  IS_DESKTOP,
  REPEAT_LABELS,
  TODAY_KEY,
  getShanghaiTodayKey,
  addMonths,
  addDays,
  dateKeyFromUTC,
  buildGridOutlinePath,
  buildMonthDays,
  calendarRange,
  colorWithAlpha,
  dateFromKey,
  isPastDateKey,
  monthFromKey,
  monthLabel,
  readableSelectionInk,
} from "../lib/calendar";

export default function CalendarWindow({
  initialView = "month",
  initialDate = TODAY_KEY,
}) {
  const [currentMonth, setCurrentMonth] = useState(() =>
    monthFromKey(initialDate),
  );
  const [view, setView] = useState(initialView);
  const [selectedDate, setSelectedDate] = useState(initialDate);
  const [dayStyle, setDayStyle] = useState("clock");
  const [saving, setSaving] = useState(false);
  const now = useNow();
  const [expandedDayKey, setExpandedDayKey] = useState(null);
  const [notice, setNotice] = useState("");
  const [desktopPicker, setDesktopPicker] = useState(null);
  const [hoveredStartDate, setHoveredStartDate] = useState(null);
  const dragSelection = useRef({ active: false, select: true, lastKey: null });
  const desktopPickerRef = useRef(null);
  const desktopPickerSessionRef = useRef(null);

  const monthDays = useMemo(() => buildMonthDays(currentMonth), [currentMonth]);
  const timelineDays = useMemo(() => {
    const date = dateFromKey(selectedDate);
    const first =
      view === "week" ? addDays(date, -(date.getUTCDay() + 6) % 7) : date;
    return Array.from({ length: view === "week" ? 7 : 1 }, (_, index) =>
      dateKeyFromUTC(addDays(first, index)),
    );
  }, [selectedDate, view]);
  const range = useMemo(
    () =>
      view === "day" || view === "week"
        ? {
            from: timelineDays[0],
            to: dateKeyFromUTC(addDays(dateFromKey(timelineDays.at(-1)), 1)),
          }
        : calendarRange(currentMonth),
    [currentMonth, view, timelineDays],
  );
  const pickingRepeat = REPEAT_LABELS[desktopPicker?.repeatMode] || "仅一次";
  const pickingDate = desktopPicker?.date || TODAY_KEY;
  const pickingCustomDates = desktopPicker?.customDates || [];
  const customDateSet = useMemo(
    () => new Set(pickingCustomDates),
    [pickingCustomDates],
  );
  const customPicking = Boolean(desktopPicker) && pickingRepeat === "自定义";
  const startDatePicking = Boolean(desktopPicker) && pickingRepeat !== "自定义";
  const calendarPicking = customPicking || startDatePicking;

  function matchesStartPattern(day, anchorDate) {
    if (!anchorDate || isPastDateKey(anchorDate) || isPastDateKey(day.key))
      return false;
    const active = dateFromKey(anchorDate);
    if (pickingRepeat === "每周")
      return dateFromKey(day.key).getUTCDay() === active.getUTCDay();
    if (pickingRepeat === "每月") return day.day === active.getUTCDate();
    return day.key === anchorDate;
  }

  function isHoveredStartPattern(day) {
    return (
      startDatePicking &&
      Boolean(hoveredStartDate) &&
      matchesStartPattern(day, hoveredStartDate)
    );
  }

  function isSelectedStartPattern(day) {
    return (
      startDatePicking &&
      Boolean(pickingDate) &&
      matchesStartPattern(day, pickingDate)
    );
  }

  const outlinedCellIndexes = monthDays.reduce((indexes, day, index) => {
    if (isPastDateKey(day.key)) return indexes;
    if (
      (customPicking && customDateSet.has(day.key)) ||
      isHoveredStartPattern(day)
    )
      indexes.push(index);
    return indexes;
  }, []);
  const selectionOutlinePath = buildGridOutlinePath(outlinedCellIndexes);

  const {
    items: events,
    loading: calendarLoading,
    error: calendarError,
    refresh: refreshCalendar,
  } = useCalendarItems(range.from, range.to);

  useEffect(() => {
    if (calendarError) setNotice(calendarError);
  }, [calendarError]);

  useEffect(
    () =>
      window.noteDesktop?.onWorkspaceViewChanged?.((payload) => {
        const date = payload?.date || getShanghaiTodayKey();
        setSelectedDate(date);
        setCurrentMonth(monthFromKey(date));
        setView(payload?.view === "day" ? "day" : "month");
      }),
    [],
  );

  function goToday() {
    const date = getShanghaiTodayKey();
    setSelectedDate(date);
    setCurrentMonth(monthFromKey(date));
  }

  async function retime(item, edge, minutes) {
    if (saving) return;
    setSaving(true);
    try {
      await retimeItem(item, edge, minutes);
    } catch (error) {
      setNotice(error.message);
    } finally {
      await refreshCalendar();
      setSaving(false);
    }
  }

  function openUtility(role) {
    const request =
      role === "create"
        ? window.noteDesktop?.openCreate?.({ date: selectedDate })
        : window.noteDesktop?.openSettings?.();
    if (request) request.catch((error) => setNotice(error.message));
    else
      window.open(
        `/?window=${role}&date=${selectedDate}`,
        "_blank",
        "width=420,height=650",
      );
  }

  useEffect(() => {
    if (
      !window.noteDesktop?.getDatePickerState ||
      !window.noteDesktop?.onDatePickerStateChanged
    ) {
      return undefined;
    }

    let disposed = false;
    function applyPickerState(state) {
      if (disposed) return;
      desktopPickerRef.current = state;
      setDesktopPicker(state);
      setHoveredStartDate(null);

      if (state && desktopPickerSessionRef.current !== state.sessionId) {
        setView("month");
        desktopPickerSessionRef.current = state.sessionId;
        setCurrentMonth(monthFromKey(state.date));
        setExpandedDayKey(null);
      }
      if (!state) desktopPickerSessionRef.current = null;
    }

    const removeListener =
      window.noteDesktop.onDatePickerStateChanged(applyPickerState);
    window.noteDesktop
      .getDatePickerState()
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
    if (view === "day" || view === "week") {
      const key = dateKeyFromUTC(
        addDays(dateFromKey(selectedDate), amount * (view === "week" ? 7 : 1)),
      );
      setSelectedDate(key);
      setCurrentMonth(monthFromKey(key));
      return;
    }
    if (view === "year") amount *= 12;
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
    if (!calendarPicking || isPastDateKey(dateKey) || event.button !== 0)
      return;
    event.preventDefault();

    if (startDatePicking) {
      const activePicker = desktopPickerRef.current;
      if (activePicker)
        publishDesktopDateSelection({
          ...activePicker,
          date: dateKey,
          customDates: [],
        });
      return;
    }

    const shouldSelect = !customDateSet.has(dateKey);
    dragSelection.current = {
      active: true,
      select: shouldSelect,
      lastKey: dateKey,
    };
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
    if (
      !calendarPicking ||
      isPastDateKey(dateKey) ||
      !["Enter", " "].includes(event.key)
    )
      return;
    event.preventDefault();
    if (startDatePicking) {
      const activePicker = desktopPickerRef.current;
      if (activePicker)
        publishDesktopDateSelection({
          ...activePicker,
          date: dateKey,
          customDates: [],
        });
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
    setSelectedDate(date);
    setCurrentMonth(monthFromKey(date));
    setView("day");
  }

  function changeView(next) {
    if (next === "week" || next === "day") {
      if (selectedDate.slice(0, 7) !== dateKeyFromUTC(currentMonth).slice(0, 7))
        setSelectedDate(dateKeyFromUTC(currentMonth));
    }
    setView(next);
  }

  function openEvent(item) {
    const request = window.noteDesktop?.openDetail?.({
      todoId: item.todoId,
      eventId: item.eventId,
      date: item.date,
    });
    if (!window.noteDesktop)
      window.open(
        `/?window=detail&${item.kind === "event" ? "event_id" : "todo_id"}=${item.eventId || item.todoId}&date=${item.date}`,
        "_blank",
        "width=360,height=650",
      );
    request?.catch((error) => setNotice(error.message));
  }

  const selectionColor = desktopPicker?.color || "#F3B51B";

  return (
    <main
      className={`calendar-shell multi-view-calendar ${IS_DESKTOP ? "is-desktop-calendar" : ""} ${calendarPicking ? "is-calendar-picking is-desktop-date-picking" : ""}`}
      style={{
        "--selection-color": selectionColor,
        "--selection-soft": colorWithAlpha(selectionColor, 0.12),
        "--selection-ink": readableSelectionInk(selectionColor),
      }}
    >
      <div className="workspace-stack">
        <div className="month-stack">
          <section
            className={`calendar-panel view-${view} ${calendarLoading ? "is-loading" : ""}`}
            aria-label={`${monthLabel(currentMonth)}日历`}
            aria-busy={calendarLoading}
          >
            <CalendarToolbar
              title={
                view === "year"
                  ? `${currentMonth.getUTCFullYear()}年`
                  : view === "day"
                    ? selectedDate.replaceAll("-", "/")
                    : view === "week"
                      ? `${timelineDays[0].slice(5).replace("-", "/")} – ${timelineDays[6].slice(5).replace("-", "/")}`
                      : monthLabel(currentMonth)
              }
              view={view}
              onView={changeView}
              onMove={moveMonth}
              onToday={goToday}
              onCreate={() => openUtility("create")}
              onSettings={() => openUtility("settings")}
              picker={desktopPicker}
              onFinish={finishDesktopDatePicking}
              dayStyle={dayStyle}
              onDayStyle={setDayStyle}
            />

            {view === "year" && (
              <YearCalendar
                year={currentMonth.getUTCFullYear()}
                onMonth={(month) => {
                  setCurrentMonth(month);
                  setView("month");
                }}
              />
            )}
            {(view === "week" ||
              (view === "day" && dayStyle === "timeline")) && (
              <CalendarTimeline
                days={timelineDays}
                items={events}
                mode={view}
                onDay={openDay}
                onTodo={openEvent}
              />
            )}
            {view === "day" && dayStyle === "clock" && (
              <DayClock
                date={selectedDate}
                items={events}
                now={now}
                onOpen={openEvent}
                onRetime={retime}
                saving={saving}
              />
            )}
            {view === "month" && (
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
            )}
          </section>
        </div>
      </div>

      {notice && (
        <div className="toast" role="status">
          {notice}
        </div>
      )}
    </main>
  );
}
