import { useEffect, useMemo, useState } from "react";
import { deleteTodo, getCalendar, patchOccurrence, patchTodo } from "../api";
import DayAgendaPanel from "../components/DayAgendaPanel";
import { calendarEventFromOccurrence, dateTimeAt, nextDateKey } from "../lib/calendar";
import { notifyDataChanged } from "../windows/window-utils";

export default function DayView({
  date,
  onOpenCreate,
  onOpenDetails,
  onOpenSettings,
  onToggleCalendar,
  calendarOpen,
}) {
  const [events, setEvents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function load(targetDate = date) {
    const result = await getCalendar(targetDate, nextDateKey(targetDate));
    setEvents(result.data.map(calendarEventFromOccurrence).sort((left, right) => left.time.localeCompare(right.time)));
  }

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    getCalendar(date, nextDateKey(date))
      .then((result) => {
        if (!active) return;
        setEvents(result.data.map(calendarEventFromOccurrence).sort((left, right) => left.time.localeCompare(right.time)));
      })
      .catch((loadError) => {
        if (active) setError(loadError.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [date]);

  useEffect(() => {
    if (!window.noteDesktop?.onDataChanged) return undefined;
    return window.noteDesktop.onDataChanged(() => load().catch((loadError) => setError(loadError.message)));
  }, [date]);

  async function saveItem(item, changes) {
    const latest = events.find((eventItem) => eventItem.todoId === item.todoId) || item;
    const payload = { version: latest.version };
    if (changes.title !== undefined) payload.title = changes.title;
    if (changes.time !== undefined) payload.starts_at = dateTimeAt(latest.startDate, changes.time);
    await patchTodo(item.todoId, payload);
    await load();
    await notifyDataChanged({ type: "updated", todoId: item.todoId });
  }

  async function completeItem(item) {
    if (item.allDone || item.occurrenceDone) return;

    const [result] = await Promise.all([
      patchOccurrence(item.todoId, item.date, true),
      new Promise((resolve) => window.setTimeout(resolve, 280)),
    ]);

    setEvents((current) => current.map((eventItem) => {
      const isTarget = eventItem.todoId === result.todo_id && eventItem.date === result.occurs_on;
      return isTarget
        ? { ...eventItem, occurrenceDone: Boolean(result.occurrence_done) }
        : eventItem;
    }));
    await notifyDataChanged({ type: "completion", todoId: item.todoId, date: item.date });
  }

  async function removeItem(item) {
    await deleteTodo(item.todoId);
    setEvents((current) => current.filter((eventItem) => eventItem.todoId !== item.todoId));
    await notifyDataChanged({ type: "deleted", todoId: item.todoId });
  }

  const content = useMemo(() => loading ? [] : events, [events, loading]);
  const completedCount = useMemo(
    () => events.filter((item) => item.allDone || item.occurrenceDone).length,
    [events],
  );
  const pendingCount = events.length - completedCount;

  useEffect(() => {
    if (loading || !window.noteDesktop?.fitDayWindow) return;
    void window.noteDesktop.fitDayWindow({
      itemCount: events.length,
      pendingCount,
      completedCount,
    });
  }, [date, events.length, pendingCount, completedCount, loading]);

  return (
    <main className="utility-window is-day-window">
      <DayAgendaPanel
        dateKey={date}
        items={content}
        onAdd={onOpenCreate}
        onToggleCalendar={onToggleCalendar}
        calendarOpen={calendarOpen}
        onOpenSettings={onOpenSettings}
        onSave={saveItem}
        onOpenDetails={onOpenDetails}
        onComplete={completeItem}
        onDelete={removeItem}
        workspaceWindow
        windowControls
      />
      {error && <div className="utility-day-error" role="alert">{error}</div>}
    </main>
  );
}
