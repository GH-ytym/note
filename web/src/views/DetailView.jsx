import { useEffect, useState } from "react";
import { ArrowsOutSimple, Trash } from "@phosphor-icons/react";
import { deleteTodo, getCalendar, getTodo, patchOccurrence, patchTodo } from "../api";
import { ColorField, SelectField, TimeField } from "../components/FormFields";
import {
  REMINDER_LABELS,
  REMINDER_VALUES,
  REPEAT_LABELS,
  REPEAT_VALUES,
  colorWithAlpha,
  dateTimeAt,
  nextDateKey,
  preciseDateLabel,
  shanghaiDateTimeParts,
  todoDateKeys,
  validDate,
} from "../lib/calendar";
import LinkedDateField from "../windows/LinkedDateField";
import WindowFrame, { LoadingWindow } from "../windows/WindowFrame";
import useLinkedDatePicker from "../windows/useLinkedDatePicker";
import { closeCurrentWindow, notifyDataChanged, windowParams } from "../windows/window-utils";

export default function DetailView({ todoId, date, onDone }) {
  const params = windowParams();
  const todoID = Number(todoId ?? params.get("todo_id"));
  const occurrenceDate = validDate(date ?? params.get("date"));
  const [record, setRecord] = useState(null);
  const [form, setForm] = useState(null);
  const [customDates, setCustomDates] = useState([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [completionSaving, setCompletionSaving] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirming, setDeleteConfirming] = useState(false);
  const [error, setError] = useState("");
  const datePicker = useLinkedDatePicker({ setForm, setCustomDates, setError });
  const finish = onDone || closeCurrentWindow;

  async function load() {
    const [todo, calendar] = await Promise.all([
      getTodo(todoID),
      getCalendar(occurrenceDate, nextDateKey(occurrenceDate)),
    ]);
    const occurrence = calendar.data.find((item) => item.todo_id === todoID);
    const startsAt = shanghaiDateTimeParts(todo.starts_at);
    setRecord({
      todo,
      occurrenceDone: Boolean(occurrence?.occurrence_done),
      allDone: Boolean(todo.all_done),
    });
    setForm({
      title: todo.title,
      content: todo.content ?? todo.title,
      repeat: REPEAT_LABELS[todo.repeat_mode] || todo.repeat_mode,
      date: startsAt.date,
      time: startsAt.time,
      reminder: REMINDER_LABELS[todo.notify_mode] || "弹窗提醒",
      color: todo.color,
    });
    setCustomDates(todoDateKeys(todo));
  }

  useEffect(() => {
    if (!Number.isSafeInteger(todoID) || todoID < 1) {
      setError("无效的日程编号");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    load()
      .catch((loadError) => setError(loadError.message))
      .finally(() => setLoading(false));
  }, [todoID, occurrenceDate]);

  useEffect(() => {
    if (!deleteConfirming) return undefined;
    const timer = window.setTimeout(() => setDeleteConfirming(false), 3500);
    return () => window.clearTimeout(timer);
  }, [deleteConfirming]);

  useEffect(() => window.noteDesktop?.onContentEditorSaved?.((result) => {
    if (Number(result?.todoId) !== todoID) return;
    setForm((current) => current ? { ...current, content: result.content } : current);
    setRecord((current) => current ? {
      ...current,
      todo: { ...current.todo, content: result.content, version: result.version },
    } : current);
    setError("");
  }), [todoID]);

  useEffect(() => {
    function receiveBrowserEditorResult(event) {
      if (event.origin !== window.location.origin || event.data?.type !== "note:content-editor-saved") return;
      const result = event.data.payload;
      if (Number(result?.todoId) !== todoID) return;
      setForm((current) => current ? { ...current, content: result.content } : current);
      setRecord((current) => current ? {
        ...current,
        todo: { ...current.todo, content: result.content, version: result.version },
      } : current);
      setError("");
    }

    window.addEventListener("message", receiveBrowserEditorResult);
    return () => window.removeEventListener("message", receiveBrowserEditorResult);
  }, [todoID]);

  function setField(name, value) {
    const nextForm = { ...form, [name]: value };
    setForm(nextForm);
    if (name === "color") void datePicker.update(nextForm, customDates);
    setError("");
  }

  function setRepeat(value) {
    const nextForm = { ...form, repeat: value };
    const nextCustomDates = value === "自定义" ? [] : customDates;
    setForm(nextForm);
    setCustomDates(nextCustomDates);
    setError("");
    void datePicker.start(nextForm, nextCustomDates);
  }

  function setNativeDate(value) {
    setField("date", value);
    if (form.repeat === "自定义") setCustomDates(value ? [value] : []);
  }

  function openContentEditor() {
    if (!record || !form) return;
    if (window.noteDesktop?.openContentEditor) {
      window.noteDesktop.openContentEditor({
        todoId: todoID,
        date: occurrenceDate,
        content: form.content,
        version: record.todo.version,
      }).catch((openError) => setError(openError.message));
      return;
    }

    const editorURL = new URL(window.location.href);
    editorURL.search = "";
    editorURL.searchParams.set("window", "content-editor");
    editorURL.searchParams.set("todo_id", String(todoID));
    editorURL.searchParams.set("date", occurrenceDate);
    window.open(editorURL, `note-content-editor-${todoID}`, "popup,width=760,height=560,resizable=yes")?.focus();
  }

  async function setOccurrenceDone(done) {
    if (!record || record.allDone) return;
    setCompletionSaving("occurrence");
    setError("");
    try {
      const result = await patchOccurrence(todoID, occurrenceDate, done);
      setRecord((current) => ({ ...current, occurrenceDone: Boolean(result.occurrence_done) }));
      await notifyDataChanged({ type: "completion", todoId: todoID, date: occurrenceDate });
    } catch (updateError) {
      setError(updateError.message);
    } finally {
      setCompletionSaving(null);
    }
  }

  async function setAllDone(allDone) {
    if (!record) return;
    setCompletionSaving("all");
    setError("");
    try {
      const updated = await patchTodo(todoID, { all_done: allDone, version: record.todo.version });
      setRecord((current) => ({ ...current, todo: updated, allDone: Boolean(updated.all_done) }));
      await notifyDataChanged({ type: "all-done", todoId: todoID });
    } catch (updateError) {
      setError(updateError.message);
    } finally {
      setCompletionSaving(null);
    }
  }

  async function submit(event) {
    event.preventDefault();
    const title = form.title.trim();
    const content = form.content.trim();
    if (!title) {
      setError("请输入标题");
      return;
    }
    if (form.repeat === "自定义" && customDates.length === 0) {
      setError("请至少添加一个自定义日期");
      return;
    }

    const startsOn = form.repeat === "自定义" ? [...customDates].sort()[0] : form.date;
    const payload = {
      title,
      content,
      color: form.color,
      starts_at: dateTimeAt(startsOn, form.time),
      repeat_mode: REPEAT_VALUES[form.repeat],
      notify_mode: REMINDER_VALUES[form.reminder],
      version: record.todo.version,
      ...(form.repeat === "自定义" ? { custom_dates: [...customDates].sort() } : {}),
    };

    setSaving(true);
    setError("");
    try {
      await patchTodo(todoID, payload);
      await notifyDataChanged({ type: "updated", todoId: todoID });
      finish();
    } catch (updateError) {
      setError(updateError.message);
    } finally {
      setSaving(false);
    }
  }

  async function removeTodo() {
    if (!deleteConfirming) {
      setDeleteConfirming(true);
      return;
    }
    setDeleting(true);
    setError("");
    try {
      await deleteTodo(todoID);
      await notifyDataChanged({ type: "deleted", todoId: todoID });
      finish();
    } catch (deleteError) {
      setDeleteConfirming(false);
      setError(deleteError.message);
    } finally {
      setDeleting(false);
    }
  }

  if (loading) return <LoadingWindow title="日程详情" onBack={onDone} />;
  if (!record || !form) {
    return <LoadingWindow title="日程详情" message={error || "没有找到这条日程"} onBack={onDone} />;
  }

  return (
    <WindowFrame title="日程详情" subtitle={preciseDateLabel(occurrenceDate)} className="is-form-window" onBack={onDone}>
      <form
        className="side-form utility-form"
        onSubmit={submit}
        style={{
          "--detail-color": form.color,
          "--detail-soft": colorWithAlpha(form.color, 0.14),
        }}
      >
        <div className="completion-controls" aria-label="完成状态">
          <label className={`completion-toggle ${record.allDone ? "is-overridden" : ""}`}>
            <input
              type="checkbox"
              checked={record.occurrenceDone}
              disabled={Boolean(completionSaving) || record.allDone}
              onChange={(event) => void setOccurrenceDone(event.target.checked)}
            />
            <span>{record.allDone ? "本次状态已保留" : record.occurrenceDone ? "本次已完成" : "本次未完成"}</span>
          </label>
          <label className={`completion-toggle completion-toggle-all ${record.allDone ? "is-active" : ""}`}>
            <input
              type="checkbox"
              checked={record.allDone}
              disabled={Boolean(completionSaving)}
              onChange={(event) => void setAllDone(event.target.checked)}
            />
            <span>{record.allDone ? "已全部完成" : "全部完成"}</span>
          </label>
        </div>

        <label>
          <span>标题</span>
          <input value={form.title} maxLength={50} required autoFocus onChange={(event) => setField("title", event.target.value)} />
        </label>

        <div className="content-editor-field">
          <label htmlFor="detail-content">内容</label>
          <div className="content-editor-field-shell">
            <textarea id="detail-content" value={form.content} rows={5} maxLength={500} onChange={(event) => setField("content", event.target.value)} />
            <button
              className="content-editor-expand-button"
              type="button"
              onClick={openContentEditor}
              aria-label="在独立的大窗口中编辑内容"
              title="专注编辑"
            >
              <ArrowsOutSimple size={18} weight="bold" aria-hidden="true" />
            </button>
          </div>
        </div>

        <SelectField label="重复" value={form.repeat} options={Object.keys(REPEAT_VALUES)} onChange={setRepeat} />

        <div className="form-row">
          <LinkedDateField
            repeat={form.repeat}
            date={form.date}
            customDates={customDates}
            color={form.color}
            active={datePicker.active}
            onOpen={() => void datePicker.start(form, customDates)}
            onNativeChange={setNativeDate}
          />
          <TimeField value={form.time} onChange={(value) => setField("time", value)} />
        </div>

        <SelectField label="提醒" value={form.reminder} options={Object.keys(REMINDER_VALUES)} onChange={(value) => setField("reminder", value)} />
        <ColorField value={form.color} onChange={(value) => setField("color", value)} />

        {error && <p className="form-error" role="alert">{error}</p>}
        <button className={`delete-button ${deleteConfirming ? "is-confirming" : ""}`} type="button" onClick={removeTodo} disabled={saving || deleting || Boolean(completionSaving)}>
          <Trash size={16} />
          {deleting ? "删除中" : deleteConfirming ? "再次点击确认删除" : "删除日程"}
        </button>
        <footer className="side-footer utility-footer">
          <button className="cancel-button" type="button" onClick={finish}>关闭</button>
          <button className="save-button" type="submit" disabled={saving || deleting || Boolean(completionSaving)}>{saving ? "保存中" : "保存"}</button>
        </footer>
      </form>
    </WindowFrame>
  );
}
