import { useEffect, useState } from "react";
import { getEvent, patchEvent } from "../api";
import { REPEAT_LABELS, shanghaiDateTimeParts } from "../lib/calendar";
import WindowFrame, { LoadingWindow } from "../windows/WindowFrame";
import { closeCurrentWindow, notifyDataChanged } from "../windows/window-utils";

const inputTime = (value) => {
  const p = shanghaiDateTimeParts(value);
  return `${p.date}T${p.time}`;
};
export default function EventDetailView({ eventId }) {
  const [record, setRecord] = useState(null),
    [form, setForm] = useState(null),
    [error, setError] = useState(""),
    [saving, setSaving] = useState(false);
  useEffect(() => {
    let active = true;
    getEvent(eventId)
      .then((item) => {
        if (active) {
          setRecord(item);
          setForm({
            title: item.title,
            content: item.content || "",
            start: inputTime(item.starts_at),
            end: inputTime(item.ends_at),
          });
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [eventId]);
  async function save(e) {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await patchEvent(eventId, {
        title: form.title,
        content: form.content,
        starts_at: new Date(`${form.start}:00+08:00`).toISOString(),
        ends_at: new Date(`${form.end}:00+08:00`).toISOString(),
        version: record.version,
      });
      await notifyDataChanged({ type: "updated", eventId });
      closeCurrentWindow();
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  }
  if (!form)
    return <LoadingWindow title="日程详情" message={error || "正在读取…"} />;
  return (
    <WindowFrame title="日程详情" className="is-form-window">
      <form className="side-form utility-form" onSubmit={save}>
        <label className="field">
          <span>标题</span>
          <input
            required
            maxLength={50}
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
          />
        </label>
        <label className="field">
          <span>内容</span>
          <textarea
            maxLength={500}
            value={form.content}
            onChange={(e) => setForm({ ...form, content: e.target.value })}
          />
        </label>
        <label className="field">
          <span>开始</span>
          <input
            required
            type="datetime-local"
            value={form.start}
            onChange={(e) => setForm({ ...form, start: e.target.value })}
          />
        </label>
        <label className="field">
          <span>结束</span>
          <input
            required
            type="datetime-local"
            value={form.end}
            onChange={(e) => setForm({ ...form, end: e.target.value })}
          />
        </label>
        <div className="event-repeat-summary">
          {REPEAT_LABELS[record.repeat_mode]}
          {record.repeat_mode !== "once" ? " · 整个周期" : ""}
        </div>
        {error && <div role="alert">{error}</div>}
        <button type="submit" className="save-button event-save-button" disabled={saving}>
          {saving ? "保存中" : "保存"}
        </button>
      </form>
    </WindowFrame>
  );
}
