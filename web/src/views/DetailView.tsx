import type { TodoDetail, ScheduleForm, CompletedUser } from "../types";
import { useEffect, useState } from "react";
import { ArrowsOutSimple, Trash } from "@phosphor-icons/react";
import { deleteTodo, getOccurrenceCompletions, getTodo, patchOccurrence, patchTodo } from "../api";
import { auth } from "../auth";
import { onDataChanged } from "../data-events";
import Avatar from "../components/Avatar";
import SchedulePermissions from "../components/SchedulePermissions";
import { useGroups } from "../groups";
import { ColorField, SelectField, TimeField } from "../components/FormFields";
import {
  REMINDER_LABELS,
  REMINDER_VALUES,
  REPEAT_LABELS,
  REPEAT_VALUES,
  colorWithAlpha,
  dateTimeAt,
  preciseDateLabel,
  shanghaiDateTimeParts,
  todoDateKeys,
  validDate,
} from "../lib/calendar";
import LinkedDateField from "../windows/LinkedDateField";
import WindowFrame, { LoadingWindow } from "../windows/WindowFrame";
import useLinkedDatePicker from "../windows/useLinkedDatePicker";
import { closeCurrentWindow, notifyDataChanged, windowParams } from "../windows/window-utils";

export default function DetailView({ todoId, date, onDone }: { todoId?: number; date?: string; onDone?: () => void }) {
  const { groups } = useGroups();
  const params = windowParams();
  const todoID = Number(todoId ?? params.get("todo_id"));
  const occurrenceDate = validDate(date ?? params.get("date"));
  const [record, setRecord] = useState<{todo: TodoDetail; occurrenceDone: boolean; users: CompletedUser[]} | null>(null);
  const [form, setForm] = useState<ScheduleForm | null>(null);
  const [customDates, setCustomDates] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [completionSaving, setCompletionSaving] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirming, setDeleteConfirming] = useState(false);
  const [error, setError] = useState("");
  const datePicker = useLinkedDatePicker({ setForm, setCustomDates, setError });
  const finish = onDone || closeCurrentWindow;
  const canEdit = record?.todo.my_role === "editor";
  const creator = record?.todo.creator_id === auth.user?.id;

  async function load() {
    const [todo, completions] = await Promise.all([
      getTodo(todoID),
      getOccurrenceCompletions(todoID, occurrenceDate),
    ]);
    const startsAt = shanghaiDateTimeParts(todo.starts_at);
    setRecord({
      todo,
      occurrenceDone: completions.users.some(user => user.id === auth.user?.id),
      users: completions.users,
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
      .catch((loadError) => setError(loadError instanceof Error ? loadError.message : "操作失败"))
      .finally(() => setLoading(false));
  }, [todoID, occurrenceDate]);

  useEffect(() => {
    if (!deleteConfirming) return undefined;
    const timer = window.setTimeout(() => setDeleteConfirming(false), 3500);
    return () => window.clearTimeout(timer);
  }, [deleteConfirming]);

  useEffect(() => {
    let active = true, generation = 0;
    const remove = onDataChanged(change => {
      if (!["permissions", "groups", "profile"].includes(change.type)) return;
      const current = ++generation;
      void getTodo(todoID).then(todo => {
        // 不覆盖未保存表单及其 version；并发编辑仍由后端乐观锁检测。
        if (active && generation === current) setRecord(record => record ? { ...record, todo: { ...record.todo, my_role: todo.my_role, member_roles: todo.member_roles, creator: todo.creator } } : record);
      }).catch(reason => {
        if (!active || generation !== current) return;
        setError(reason instanceof Error ? reason.message : "读取权限失败");
        if (reason?.status === 403 || reason?.status === 404) setRecord(null);
      });
    });
    return () => { active = false; remove(); };
  }, [todoID]);

  useEffect(() => window.noteDesktop?.onContentEditorSaved?.((result) => {
    if (Number(result?.todoId) !== todoID) return;
    setForm((current) => current ? { ...current, content: result.content ?? "" } : current);
    setRecord((current) => current ? {
      ...current,
      todo: { ...current.todo, content: result.content, version: result.version },
    } : current);
    setError("");
  }), [todoID]);

  useEffect(() => {
    function receiveBrowserEditorResult(event: MessageEvent) {
      if (event.origin !== window.location.origin || event.data?.type !== "note:content-editor-saved") return;
      const result = event.data.payload;
      if (Number(result?.todoId) !== todoID) return;
      setForm((current) => current ? { ...current, content: result.content ?? "" } : current);
      setRecord((current) => current ? {
        ...current,
        todo: { ...current.todo, content: result.content, version: result.version },
      } : current);
      setError("");
    }

    window.addEventListener("message", receiveBrowserEditorResult);
    return () => window.removeEventListener("message", receiveBrowserEditorResult);
  }, [todoID]);

  function setField(name: keyof ScheduleForm, value: string) {
    if (!form) return;
    const nextForm = { ...form, [name]: value };
    setForm(nextForm);
    if (name === "color") void datePicker.update(nextForm, customDates);
    setError("");
  }

  function setRepeat(value: string) {
    if (!form) return;
    const nextForm = { ...form, repeat: value };
    const nextCustomDates = value === "自定义" ? [] : customDates;
    setForm(nextForm);
    setCustomDates(nextCustomDates);
    setError("");
    void datePicker.start(nextForm, nextCustomDates);
  }

  function setNativeDate(value: string) {
    if (!form) return;
    setField("date", value);
    if (form.repeat === "自定义") setCustomDates(value ? [value] : []);
  }

  function openContentEditor() {
    if (!record || !form || !canEdit) return;
    if (window.noteDesktop?.openContentEditor) {
      window.noteDesktop.openContentEditor({
        todoId: todoID,
        date: occurrenceDate,
        content: form.content,
        version: record.todo.version,
      }).catch((openError) => setError(openError instanceof Error ? openError.message : "操作失败"));
      return;
    }

    const editorURL = new URL(window.location.href);
    editorURL.search = "";
    editorURL.searchParams.set("window", "content-editor");
    editorURL.searchParams.set("todo_id", String(todoID));
    editorURL.searchParams.set("date", occurrenceDate);
    window.location.assign(editorURL);
  }

  async function setOccurrenceDone(done: boolean) {
    if (!record) return;
    setCompletionSaving("occurrence");
    setError("");
    try {
      await patchOccurrence(todoID, occurrenceDate, done);
      setRecord((current) => current ? ({ ...current, occurrenceDone: done }) : current);
      await notifyDataChanged({ type: "completion", todoId: todoID, date: occurrenceDate });
      const completions = await getOccurrenceCompletions(todoID, occurrenceDate);
      setRecord((current) => current ? ({ ...current,
        occurrenceDone: completions.users.some(user => user.id === auth.user?.id),
        users: completions.users,
      }) : current);
    } catch (updateError) {
      setError(updateError instanceof Error ? updateError.message : "操作失败");
    } finally {
      setCompletionSaving(null);
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!form || !record || !canEdit) return;
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
      setError(updateError instanceof Error ? updateError.message : "操作失败");
    } finally {
      setSaving(false);
    }
  }

  async function removeTodo() {
    if (!creator) return;
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
      setError(deleteError instanceof Error ? deleteError.message : "操作失败");
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
        <div className="group-context"><span>{groups.find(group => group.id === record.todo.group_id)?.name || "群组"}</span>{record.todo.creator && <><Avatar url={record.todo.creator.avatar} name={record.todo.creator.nickname || record.todo.creator.username} size={24} /><span>{record.todo.creator.nickname || record.todo.creator.username} 创建</span></>}<small>{canEdit ? "可编辑" : "只读"}</small></div>
        <div className="completion-controls" aria-label="完成状态">
          <label className="completion-toggle">
            <input
              type="checkbox"
              checked={record.occurrenceDone}
              disabled={Boolean(completionSaving)}
              onChange={(event) => void setOccurrenceDone(event.target.checked)}
            />
            <span>{record.occurrenceDone ? "我已完成本次 Todo" : "我尚未完成本次 Todo"}</span>
          </label>
        </div>
        <section aria-label="本次完成名单">
          <p>已完成：{record.users.length} 人</p>
          {record.users.length === 0 ? <p>还没有人完成</p> : (
            <ul>
              {record.users.map(user => (
                <li key={user.id} className="completion-user">
                  <Avatar url={user.avatar} name={user.nickname || user.username} size={26} />
                  {user.nickname || user.username || `用户 ${user.id}`} · {new Date(user.completed_at).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai" })}
                </li>
              ))}
            </ul>
          )}
        </section>

        <fieldset className="schedule-fields" disabled={!canEdit || saving}>
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

        </fieldset>
        {creator && <SchedulePermissions kind="todo" id={todoID} groupID={record.todo.group_id} creatorID={record.todo.creator_id} roles={record.todo.member_roles} onChanged={async () => { const todo = await getTodo(todoID); setRecord(current => current ? { ...current, todo: { ...current.todo, my_role: todo.my_role, member_roles: todo.member_roles } } : current); }} />}
        {error && <p className="form-error" role="alert">{error}</p>}
        {creator && <button className={`delete-button ${deleteConfirming ? "is-confirming" : ""}`} type="button" onClick={removeTodo} disabled={saving || deleting || Boolean(completionSaving)}>
          <Trash size={16} />
          {deleting ? "删除中" : deleteConfirming ? "再次点击确认删除" : "删除日程"}
        </button>}
        <footer className="side-footer utility-footer">
          <button className="cancel-button" type="button" onClick={finish}>关闭</button>
          {canEdit && <button className="save-button" type="submit" disabled={saving || deleting || Boolean(completionSaving)}>{saving ? "保存中" : "保存"}</button>}
        </footer>
      </form>
    </WindowFrame>
  );
}
