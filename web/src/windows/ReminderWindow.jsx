import { useEffect, useRef, useState } from "react";
import { ArrowSquareOut, BellRinging } from "@phosphor-icons/react";
import { colorWithAlpha, shanghaiDateTimeParts, validDate } from "../lib/calendar";
import WindowFrame, { LoadingWindow } from "./WindowFrame";
import { closeCurrentWindow, windowParams } from "./window-utils";

export default function ReminderWindow() {
  const [reminder, setReminder] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const detailButtonRef = useRef(null);

  useEffect(() => {
    const params = windowParams();
    const fallbackTodoID = Number(params.get("todo_id"));
    const fallbackOccursAt = params.get("occurs_at");
    const fallback = Number.isSafeInteger(fallbackTodoID)
      && fallbackTodoID > 0
      && !Number.isNaN(Date.parse(fallbackOccursAt))
      ? {
          todoId: fallbackTodoID,
          title: params.get("title") || params.get("content") || "日程到点了",
          content: params.get("content") || "日程到点了",
          color: params.get("color") || "#F3B51B",
          occursAt: fallbackOccursAt,
          date: validDate(params.get("date")),
          notifyMode: "popup",
        }
      : null;
    const request = window.noteDesktop?.getReminderState?.();

    Promise.resolve(request || fallback)
      .then((state) => {
        if (!state) throw new Error("提醒内容已经失效");
        setReminder(state);
      })
      .catch((loadError) => setError(loadError.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (reminder) detailButtonRef.current?.focus();
  }, [reminder]);

  async function openReminderDetail() {
    if (!reminder) return;

    if (window.noteDesktop?.openDetail) {
      try {
        await window.noteDesktop.openDetail({ todoId: reminder.todoId, date: reminder.date });
        closeCurrentWindow();
      } catch (openError) {
        setError(openError.message);
      }
      return;
    }

    const detailURL = new URL(window.location.href);
    detailURL.search = "";
    detailURL.searchParams.set("window", "detail");
    detailURL.searchParams.set("todo_id", String(reminder.todoId));
    detailURL.searchParams.set("date", reminder.date);
    window.location.assign(detailURL);
  }

  if (loading) return <LoadingWindow title="日程提醒" />;
  if (!reminder) return <LoadingWindow title="日程提醒" message={error || "提醒内容已经失效"} />;

  const occursAt = shanghaiDateTimeParts(reminder.occursAt);

  return (
    <WindowFrame title="提醒" className="is-reminder-window" closeLabel="关闭提醒">
      <section className="reminder-dialog" role="alertdialog" aria-label="日程到点提醒">
        <div
          className="reminder-summary"
          style={{
            "--reminder-color": reminder.color || "#F3B51B",
            "--reminder-soft": colorWithAlpha(reminder.color || "#F3B51B", 0.12),
          }}
        >
          <span className="reminder-bell" aria-hidden="true">
            <BellRinging size={27} weight="fill" />
          </span>
          <div>
            <time className="reminder-time" dateTime={reminder.occursAt}>{occursAt.time}</time>
            <p>{reminder.title || reminder.content}</p>
          </div>
        </div>

        {error && <p className="reminder-error" role="alert">{error}</p>}

        <footer className="reminder-actions">
          <button className="reminder-dismiss" type="button" onClick={closeCurrentWindow}>关闭</button>
          <button ref={detailButtonRef} className="reminder-open" type="button" onClick={openReminderDetail}>
            查看
            <ArrowSquareOut size={17} weight="bold" aria-hidden="true" />
          </button>
        </footer>
      </section>
    </WindowFrame>
  );
}
