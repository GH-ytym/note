import CreateView from "./CreateView";
import { openPage } from "../components/Workspace";
import type { View, CalendarItem, SearchItem } from "../types";
import type { PickerState, WorkspaceState } from "../desktop";
type MonthDay = {key: string; day: number; isCurrentMonth: boolean};
import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowsOut } from "@phosphor-icons/react";
import CalendarToolbar from "../components/CalendarToolbar";
import DayClock from "../components/DayClock";
import useNow from "../hooks/useNow";
import useCalendarItems from "../hooks/useCalendarItems";
import { useAppearance } from "../appearance";
import { retimeItem } from "../lib/retime";
import CalendarGrid from "../components/CalendarGrid";
import YearCalendar from "../components/YearCalendar";
import CalendarTimeline from "../components/CalendarTimeline";
import SearchBox from "../components/SearchBox";
import GroupSidebar from "../components/GroupSidebar";
import { useGroups } from "../groups";
import DayAgendaPanel from "../components/DayAgendaPanel";
import MiniDragHandle from "../components/MiniDragHandle";
import { assignEventTracks } from "../lib/timeline";
import "../styles/search.css";
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
  active = true,
  initialView,
  onMiniChange,
  onPickingChange,
  initialDate = TODAY_KEY,
}: { active?: boolean; initialView?: View; initialDate?: string; onPickingChange?: (picking: boolean) => void; onMiniChange?: (mini: boolean) => void } = {}) {
  const { settings } = useAppearance();
  const { selected: selectedGroup } = useGroups();
  const [currentMonth, setCurrentMonth] = useState(() =>
    monthFromKey(initialDate),
  );
  const [view, setView] = useState(() => initialView || settings.defaultView);
  const [selectedDate, setSelectedDate] = useState(initialDate);
  const [dayStyle, setDayStyle] = useState(settings.dayViewMode);
  const [handMode, setHandMode] = useState(settings.handMode);
  const [saving, setSaving] = useState(false);
  const [mini, setMini] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createDate, setCreateDate] = useState(initialDate);
  const localPicker = useRef(false);
  useEffect(() => { onMiniChange?.(mini); }, [mini, onMiniChange]);
  useEffect(() => {
    const receive = (event: Event) => {
      const state = (event as CustomEvent<PickerState | null>).detail;
      localPicker.current = !!state;
      desktopPickerRef.current = state; setDesktopPicker(state);
      if (state) { setView("month"); setCurrentMonth(monthFromKey(state.date)); }
    };
    window.addEventListener("note-inline-picker", receive);
    return () => window.removeEventListener("note-inline-picker", receive);
  }, []);
  function closeCreate() {
    setCreating(false);
    localPicker.current = false;
    desktopPickerRef.current = null; setDesktopPicker(null);
  }
  const [workspaceReady, setWorkspaceReady] = useState(!IS_DESKTOP);
  const now = useNow();
  const eventTracks = useRef(new Map<string, number>());
  const [notice, setNotice] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchTarget, setSearchTarget] = useState<{kind: SearchItem["kind"]; id: number; nonce: number} | null>(null);
  const browserAuxiliary = useRef<Window | null>(null);
  const [desktopPicker, setDesktopPicker] = useState<PickerState | null>(null);
  const [hoveredStartDate, setHoveredStartDate] = useState<string | null>(null);
  const [todayPulse, setTodayPulse] = useState(0);
  const dragSelection = useRef({ active: false, select: true, lastKey: null as string | null });
  const desktopPickerRef = useRef<PickerState | null>(null);
  const desktopPickerSessionRef = useRef<number | null>(null);

  function changeSearchOpen(open: boolean) {
    setSearchOpen(open);
    if (open) {
      browserAuxiliary.current?.close();
      browserAuxiliary.current = null;
      void window.noteDesktop?.claimInlinePanel?.();
    }
  }

  useEffect(() => window.noteDesktop?.onAuxiliaryOpened?.(() => setSearchOpen(false)), []);

  function selectSearchResult(item: SearchItem) {
    const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date(item.starts_at));
    const part = (type: string) => parts.find(value => value.type === type)?.value;
    const date = `${part("year")}-${part("month")}-${part("day")}`;
    setSelectedDate(date);
    setCurrentMonth(monthFromKey(date));
    if (view === "year") setView("month");
    setSearchTarget({ kind: item.kind, id: item.id, nonce: Date.now() });
  }

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
            to: dateKeyFromUTC(addDays(dateFromKey(timelineDays[timelineDays.length - 1]), 1)),
          }
        : calendarRange(currentMonth),
    [currentMonth, view, timelineDays],
  );
  const pickingRepeat = REPEAT_LABELS[desktopPicker?.repeatMode || "once"] || "仅一次";
  const pickingDate = desktopPicker?.date || TODAY_KEY;
  const pickingCustomDates = desktopPicker?.customDates || [];
  const customDateSet = useMemo(
    () => new Set(pickingCustomDates),
    [pickingCustomDates],
  );
  const customPicking = Boolean(desktopPicker) && pickingRepeat === "自定义";
  const startDatePicking = Boolean(desktopPicker) && pickingRepeat !== "自定义";
  const calendarPicking = customPicking || startDatePicking;
  useEffect(() => { onPickingChange?.(calendarPicking); return () => onPickingChange?.(false); }, [calendarPicking, onPickingChange]);
  useEffect(() => {
    if (!calendarPicking) return;
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") { event.preventDefault(); finishDesktopDatePicking(); } };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, [calendarPicking]);

  function matchesStartPattern(day: MonthDay, anchorDate: string | null) {
    if (!anchorDate || isPastDateKey(anchorDate) || isPastDateKey(day.key))
      return false;
    const active = dateFromKey(anchorDate);
    if (pickingRepeat === "每周")
      return dateFromKey(day.key).getUTCDay() === active.getUTCDay();
    if (pickingRepeat === "每月") return day.day === active.getUTCDate();
    return day.key === anchorDate;
  }

  function isHoveredStartPattern(day: MonthDay) {
    return (
      startDatePicking &&
      Boolean(hoveredStartDate) &&
      matchesStartPattern(day, hoveredStartDate)
    );
  }

  function isSelectedStartPattern(day: MonthDay) {
    return (
      startDatePicking &&
      Boolean(pickingDate) &&
      matchesStartPattern(day, pickingDate)
    );
  }

  const outlinedCellIndexes = monthDays.reduce<number[]>((indexes, day, index) => {
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
    items: rawEvents,
    loading: calendarLoading,
    error: calendarError,
    refresh: refreshCalendar,
  } = useCalendarItems(range.from, range.to, selectedGroup?.id ?? null, active);
  const events = useMemo(() => assignEventTracks(rawEvents, eventTracks.current), [rawEvents]);

  useEffect(() => {
    if (!searchTarget || calendarLoading) return;
    const selector = `[data-search-key="${searchTarget.kind}-${searchTarget.id}"]`;
    const elements = [...document.querySelectorAll(selector)];
    if (!elements.length) return;
    elements[0].scrollIntoView({ block: "nearest", inline: "nearest" });
    for (const element of elements) {
      element.classList.remove("search-focus-target");
      void element.getBoundingClientRect();
      element.classList.add("search-focus-target");
    }
    const timer = setTimeout(() => { elements.forEach(element => element.classList.remove("search-focus-target")); setSearchTarget(null); }, 1450);
    return () => { clearTimeout(timer); elements.forEach(element => element.classList.remove("search-focus-target")); };
  }, [searchTarget, calendarLoading, events]);

  useEffect(() => {
    if (calendarError) setNotice(calendarError);
  }, [calendarError]);

  useEffect(() => {
    setDayStyle(mini ? settings.miniViewMode : settings.dayViewMode);
    setHandMode(settings.handMode);
  }, [settings.dayViewMode, settings.miniViewMode, settings.handMode, mini]);

  useEffect(() => {
    let disposed = false;
    const apply = (payload: WorkspaceState) => {
        if (!payload || disposed) return;
        const date = payload?.date || getShanghaiTodayKey();
        setSelectedDate(date);
        setCurrentMonth(monthFromKey(date));
        setView(payload.view || "month");
        setMini(Boolean(payload.mini));
        if (payload.dayStyle) setDayStyle(payload.dayStyle);
        if (payload.handMode) setHandMode(payload.handMode);
        setWorkspaceReady(true);
    };
    const remove = window.noteDesktop?.onWorkspaceViewChanged?.(apply);
    window.noteDesktop?.getWorkspaceState?.().then(apply).catch(error => setNotice(error instanceof Error ? error.message : "操作失败"));
    return () => { disposed = true; remove?.(); };
  }, []);

  useEffect(() => {
    if (workspaceReady && !desktopPicker) void window.noteDesktop?.saveWorkspaceState?.({ view, date: selectedDate, dayStyle, handMode, mini });
  }, [workspaceReady, view, selectedDate, dayStyle, handMode, mini, desktopPicker]);

  useEffect(() => {
    if (mini) { setSelectedDate(now.date); setCurrentMonth(monthFromKey(now.date)); }
  }, [mini, now.date]);

  function changeMini(next: boolean) {
    if (window.noteDesktop?.setMiniMode) void window.noteDesktop.setMiniMode(next);
    else { setMini(next); if (next) { setView("day"); setDayStyle(settings.miniViewMode); setSelectedDate(now.date); } }
  }

  function goToday() {
    const date = getShanghaiTodayKey();
    setSelectedDate(date);
    setCurrentMonth(monthFromKey(date));
    if (view === "year") setTodayPulse((value) => value + 1);
  }

  async function retime(item: CalendarItem, edge: "start" | "end", minutes: number) {
    if (item.canEdit === false) return;
    if (saving) return;
    setSaving(true);
    try {
      await retimeItem(item, edge, minutes);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "操作失败");
    } finally {
      await refreshCalendar();
      setSaving(false);
    }
  }

  function openUtility(role: "create" | "settings") {
    if (role === "create" && !selectedGroup) { setNotice("请先创建、加入或选择一个群组"); return; }
    setSearchOpen(false);
    if (role === "create") { setCreateDate(selectedDate); setCreating(true); return; }
    openPage({id: role, title: role === "settings" ? "设置" : "新建", date: selectedDate});
  }

  useEffect(() => {
    if (
      !window.noteDesktop?.getDatePickerState ||
      !window.noteDesktop?.onDatePickerStateChanged
    ) {
      return undefined;
    }

    let disposed = false;
    function applyPickerState(state: PickerState | null) {
      if (disposed) return;
      desktopPickerRef.current = state;
      setDesktopPicker(state);
      setHoveredStartDate(null);

      if (state && desktopPickerSessionRef.current !== state.sessionId) {
        setMini(false);
        setView("month");
        desktopPickerSessionRef.current = state.sessionId ?? null;
        setCurrentMonth(monthFromKey(state.date));
      }
      if (!state) desktopPickerSessionRef.current = null;
    }

    const removeListener =
      window.noteDesktop.onDatePickerStateChanged(applyPickerState);
    window.noteDesktop
      .getDatePickerState()
      .then(applyPickerState)
      .catch((error) => setNotice(error instanceof Error ? error.message : "操作失败"));

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

  function moveMonth(amount: number) {
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
  }

  function publishDesktopDateSelection(nextState: PickerState) {
    desktopPickerRef.current = nextState;
    setDesktopPicker(nextState);
    if (localPicker.current) { window.dispatchEvent(new CustomEvent("note-inline-selection", {detail: nextState})); return; }
    const request = window.noteDesktop?.selectDatePicker?.({
      date: nextState.date,
      customDates: nextState.customDates,
    });
    request?.catch((error) => setNotice(error instanceof Error ? error.message : "操作失败"));
  }

  function updateCustomDate(dateKey: string, shouldSelect: boolean) {
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

  function beginDateSelection(dateKey: string, event: React.PointerEvent) {
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

  function continueDateSelection(dateKey: string, pastDateDisabled: boolean) {
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

  function toggleDateFromKeyboard(dateKey: string, event: React.KeyboardEvent) {
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
    if (localPicker.current) { window.dispatchEvent(new CustomEvent("note-inline-picker", {detail:null})); window.dispatchEvent(new Event("note-inline-finished")); return; }
    const request = window.noteDesktop?.finishDatePicker?.();
    request?.catch((error) => setNotice(error instanceof Error ? error.message : "操作失败"));
  }

  function openDay(date: string) {
    setSelectedDate(date);
    setCurrentMonth(monthFromKey(date));
    setView("day");
  }

  function changeView(next: View) {
    if (next === "week" || next === "day") {
      if (selectedDate.slice(0, 7) !== dateKeyFromUTC(currentMonth).slice(0, 7))
        setSelectedDate(dateKeyFromUTC(currentMonth));
    }
    setView(next);
  }

  function openEvent(item: CalendarItem) {
    setSearchOpen(false);
    openPage({id: `${item.kind}-${item.eventId || item.todoId}-${item.date}`, title: "日程详情", todoId: item.todoId, eventId: item.eventId, date: item.date});
  }

  const selectionColor = desktopPicker?.color || "#F3B51B";

  return (
    <main
      className={`calendar-shell multi-view-calendar ${!mini ? "has-group-sidebar" : ""} ${mini ? "is-mini-clock" : ""} ${IS_DESKTOP ? "is-desktop-calendar" : ""} ${calendarPicking ? "is-calendar-picking is-desktop-date-picking" : ""}`}
      style={{
        "--selection-color": selectionColor,
        "--selection-soft": colorWithAlpha(selectionColor, 0.12),
        "--selection-ink": readableSelectionInk(selectionColor),
      }}
    >
      {!mini && <GroupSidebar disabled={calendarPicking} onSettings={() => openUtility("settings")} />}
      <div className="workspace-stack">
        <div className="month-stack">
          <section
            className={`calendar-panel view-${view} ${calendarLoading ? "is-loading" : ""}`}
            aria-label={`${monthLabel(currentMonth)}日历`}
            aria-busy={calendarLoading}
          >
            {mini ? <><MiniDragHandle /><button className="mini-restore" onClick={() => changeMini(false)} title="恢复主面板" aria-label="恢复主面板"><ArrowsOut size={18} /></button></> : <CalendarToolbar
              search={!calendarPicking && <SearchBox open={searchOpen} onOpen={changeSearchOpen} onSelect={selectSearchResult} limit={settings.searchLimit} defaultSplit={settings.searchSplit} />}
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
                  picker={desktopPicker}
              onFinish={finishDesktopDatePicking}
              dayStyle={dayStyle}
              onDayStyle={setDayStyle}
              onMini={() => changeMini(true)}
            />}

            {view === "day" && dayStyle === "tags" && <DayAgendaPanel date={selectedDate} items={events} onOpen={openEvent} onRefresh={refreshCalendar} />}
            {view === "year" && (
              <YearCalendar
                year={currentMonth.getUTCFullYear()}
                focusMonth={Number(getShanghaiTodayKey().slice(5, 7)) - 1}
                focusPulse={todayPulse}
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
                trackCount={settings.clockTracks}
                orientation={
                  view === "week"
                    ? settings.weekOrientation
                    : settings.dayOrientation
                }
                onDay={openDay}
                onTodo={openEvent}
                onRetime={retime}
                saving={saving}
              />
            )}
            {view === "day" && dayStyle === "clock" && (
              <DayClock
                date={selectedDate}
                items={events}
                now={now}
                handMode={handMode}
                trackCount={settings.clockTracks}
                onOpen={openEvent}
                mini={mini}
                onRetime={retime}
                saving={saving}
              />
            )}
            {view === "month" && (
              <CalendarGrid
                days={monthDays}
                events={events}
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

      {creating && <aside className="create-sidebar" inert={calendarPicking} hidden={mini} aria-label="新建日程"><CreateView initialDate={createDate} onDone={closeCreate} inline /></aside>}
      {notice && (
        <div className="toast" role="status">
          {notice}
        </div>
      )}
    </main>
  );
}
