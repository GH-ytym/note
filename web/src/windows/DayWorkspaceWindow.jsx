import { useEffect, useState } from "react";
import { TODAY_KEY, validDate } from "../lib/calendar";
import CreateView from "../views/CreateView";
import DayView from "../views/DayView";
import DetailView from "../views/DetailView";
import SettingsView from "../views/SettingsView";
import { windowParams } from "./window-utils";

function dayState(date) {
  return { name: "day", date: validDate(date) };
}

function normalizeWorkspaceView(payload, fallbackDate) {
  const name = String(payload?.view || payload?.name || "day");
  const date = validDate(payload?.date || fallbackDate);
  if (name === "detail") {
    const todoId = Number(payload?.todoId);
    if (Number.isSafeInteger(todoId) && todoId > 0) return { name, date, todoId };
  }
  if (name === "create" || name === "settings") return { name, date };
  return dayState(date);
}

export default function DayWorkspaceWindow() {
  const initialDate = validDate(windowParams().get("date") || TODAY_KEY);
  const [date, setDate] = useState(initialDate);
  const [view, setView] = useState(() => dayState(initialDate));
  const [calendarOpen, setCalendarOpen] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!window.noteDesktop?.onDayDateChanged) return undefined;
    return window.noteDesktop.onDayDateChanged((payload) => {
      const nextDate = validDate(payload?.date);
      const nextURL = new URL(window.location.href);
      nextURL.searchParams.set("date", nextDate);
      window.history.replaceState(null, "", nextURL);
      document.title = `${nextDate} · Note`;
      setDate(nextDate);
      setView(dayState(nextDate));
      setError("");
    });
  }, []);

  useEffect(() => {
    if (!window.noteDesktop?.onWorkspaceViewChanged) return undefined;
    return window.noteDesktop.onWorkspaceViewChanged((payload) => {
      const next = normalizeWorkspaceView(payload, date);
      setDate(next.date);
      setView(next);
      setError("");
    });
  }, [date]);

  useEffect(() => {
    let disposed = false;
    const removeListener = window.noteDesktop?.onCalendarVisibilityChanged?.((payload) => {
      if (!disposed) setCalendarOpen(Boolean(payload?.open));
    });
    window.noteDesktop?.getCalendarVisibility?.()
      .then((payload) => {
        if (!disposed) setCalendarOpen(Boolean(payload?.open));
      })
      .catch(() => {});
    return () => {
      disposed = true;
      removeListener?.();
    };
  }, []);

  async function request(action, fallback) {
    fallback();
    try {
      await action?.();
    } catch (requestError) {
      setError(requestError.message);
    }
  }

  function openCreate() {
    const next = { name: "create", date };
    void request(() => window.noteDesktop?.openCreate?.({ date }), () => setView(next));
  }

  function openDetails(item) {
    const next = { name: "detail", date: item.date, todoId: item.todoId };
    void request(
      () => window.noteDesktop?.openDetail?.({ todoId: item.todoId, date: item.date }),
      () => setView(next),
    );
  }

  function openSettings() {
    const next = { name: "settings", date };
    void request(() => window.noteDesktop?.openSettings?.(), () => setView(next));
  }

  function toggleCalendar() {
    const previous = calendarOpen;
    setCalendarOpen(!previous);
    window.noteDesktop?.toggleCalendar?.()
      .then((payload) => setCalendarOpen(Boolean(payload?.open)))
      .catch((requestError) => {
        setCalendarOpen(previous);
        setError(requestError.message);
      });
  }

  function returnToDay() {
    setView(dayState(date));
    const request = window.noteDesktop?.openDay?.({ date });
    request?.catch((requestError) => setError(requestError.message));
  }

  let content;
  if (view.name === "create") {
    content = <CreateView key={`create:${view.date}`} initialDate={view.date} onDone={returnToDay} />;
  } else if (view.name === "detail") {
    content = (
      <DetailView
        key={`detail:${view.todoId}:${view.date}`}
        todoId={view.todoId}
        date={view.date}
        onDone={returnToDay}
      />
    );
  } else if (view.name === "settings") {
    content = <SettingsView onDone={returnToDay} />;
  } else {
    content = (
      <DayView
        date={date}
        onOpenCreate={openCreate}
        onOpenDetails={openDetails}
        onOpenSettings={openSettings}
        onToggleCalendar={toggleCalendar}
        calendarOpen={calendarOpen}
      />
    );
  }

  return (
    <div className={`workspace-day-view is-${view.name}`}>
      {content}
      {error && <div className="utility-day-error" role="alert">{error}</div>}
    </div>
  );
}
