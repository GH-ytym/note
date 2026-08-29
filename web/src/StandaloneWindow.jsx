import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowsOutSimple, ArrowSquareOut, BellRinging, CalendarBlank, Check, CheckCircle, Palette, Sparkle, Trash } from "@phosphor-icons/react";
import { createTodo, deleteTodo, getCalendar, getTodo, patchOccurrence, patchTodo } from "./api";
import { AppearanceSettingsForm } from "./appearance";
import {
  CustomColorSwatch,
  DayAgendaPanel,
  EVENT_COLORS,
  REMINDER_LABELS,
  REMINDER_VALUES,
  REPEAT_LABELS,
  REPEAT_VALUES,
  SelectField,
  TODAY_KEY,
  TimeField,
  WindowControls,
  addDays,
  calendarEventFromOccurrence,
  colorWithAlpha,
  dateFromKey,
  dateKeyFromUTC,
  dateTimeAt,
  preciseDateLabel,
  shanghaiDateTimeParts,
} from "./App";

const params = new URLSearchParams(window.location.search);

function closeCurrentWindow() {
  if (window.noteDesktop?.closeCurrent) {
    void window.noteDesktop.closeCurrent();
    return;
  }
  window.close();
}

async function notifyDataChanged(payload) {
  await window.noteDesktop?.notifyDataChanged?.(payload);
}

function validDate(value) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value || "") ? value : TODAY_KEY;
}

function nextDateKey(value) {
  return dateKeyFromUTC(addDays(dateFromKey(value), 1));
}

function todoDateKeys(todo) {
  return (todo.custom_dates || [])
    .map((item) => String(item.date || "").slice(0, 10))
    .filter((date) => /^\d{4}-\d{2}-\d{2}$/.test(date))
    .sort();
}

function WindowFrame({ title, subtitle, children, className = "", closeLabel = "关闭当前窗口" }) {
  return (
    <main className={`utility-window ${className}`}>
      <section className="utility-panel">
        <header className="side-header utility-header">
          <div>
            <h1>{title}</h1>
            {subtitle && <small>{subtitle}</small>}
          </div>
          <WindowControls closeLabel={closeLabel} />
        </header>
        {children}
      </section>
    </main>
  );
}

function ColorField({ value, onChange, allowRandom = false }) {
  const [open, setOpen] = useState(false);
  const selected = EVENT_COLORS.find((item) => item.value === value);
  const custom = Boolean(value) && !selected;

  return (
    <div className="color-field">
      <span className="field-label">颜色</span>
      <button
        className="color-trigger"
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <span
          className={`color-trigger-swatch ${value ? "" : "is-random"}`}
          style={value ? { background: value } : undefined}
        />
        <span>{selected?.label || (value ? "自定义" : "随机")}</span>
        <Palette size={17} />
      </button>

      {open && (
        <div className="color-palette">
          <div className="palette-grid">
            {allowRandom && (
              <button
                className={`palette-swatch random-swatch ${value ? "" : "is-selected"}`}
                type="button"
                onClick={() => onChange("")}
                aria-label="随机颜色"
              >
                <Sparkle size={14} weight="fill" />
              </button>
            )}
            {EVENT_COLORS.map((color) => (
              <button
                className={`palette-swatch ${value === color.value ? "is-selected" : ""}`}
                type="button"
                style={{ "--swatch-color": color.value }}
                onClick={() => onChange(color.value)}
                aria-label={color.label}
                key={color.value}
              >
                {value === color.value && <Check size={12} weight="bold" />}
              </button>
            ))}
            <CustomColorSwatch value={value} selected={custom} onChange={onChange} />
          </div>
        </div>
      )}
    </div>
  );
}

function pickerPayload(form, customDates) {
  return {
    repeatMode: REPEAT_VALUES[form.repeat],
    date: form.date,
    customDates: form.repeat === "自定义" ? customDates : [],
    color: form.color,
  };
}

function useLinkedDatePicker({ setForm, setCustomDates, setError }) {
  const [active, setActive] = useState(false);

  useEffect(() => {
    const removeSelectionListener = window.noteDesktop?.onDatePickerSelection?.((selection) => {
      if (!selection) return;
      setForm((current) => current ? { ...current, date: validDate(selection.date) } : current);
      setCustomDates(Array.isArray(selection.customDates) ? selection.customDates : []);
      setError("");
    });
    const removeFinishedListener = window.noteDesktop?.onDatePickerFinished?.(() => setActive(false));

    return () => {
      removeSelectionListener?.();
      removeFinishedListener?.();
    };
  }, [setCustomDates, setError, setForm]);

  async function start(form, customDates) {
    if (!window.noteDesktop?.startDatePicker) return;
    try {
      setActive(true);
      await window.noteDesktop.startDatePicker(pickerPayload(form, customDates));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  async function update(form, customDates) {
    if (!active || !window.noteDesktop?.updateDatePicker) return;
    try {
      await window.noteDesktop.updateDatePicker(pickerPayload(form, customDates));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  return { active, start, update };
}

function LinkedDateField({ repeat, date, customDates, color, active, onOpen, onNativeChange }) {
  const custom = repeat === "自定义";
  const summary = custom
    ? customDates.length > 0 ? `已选 ${customDates.length} 天` : "尚未选择"
    : date;

  return (
    <div className="linked-date-field" style={{ "--linked-date-color": color || "#F3B51B" }}>
      <span>{custom ? "日期" : "开始日期"}</span>
      {window.noteDesktop?.isDesktop ? (
        <button type="button" onClick={onOpen} aria-label="在主日历中选择日期">
          <CalendarBlank size={17} />
          <strong>{summary}</strong>
          <small>{active ? "正在主日历选择" : "去主日历选择"}</small>
          <ArrowSquareOut size={16} />
        </button>
      ) : (
        <input type="date" min={TODAY_KEY} value={date} onChange={(event) => onNativeChange(event.target.value)} />
      )}
    </div>
  );
}

function CreateWindow() {
  const initialDate = validDate(params.get("date"));
  const [form, setForm] = useState({
    content: "",
    repeat: "仅一次",
    date: initialDate,
    time: "09:00",
    reminder: "弹窗提醒",
    color: "",
  });
  const [customDates, setCustomDates] = useState([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const datePicker = useLinkedDatePicker({ setForm, setCustomDates, setError });

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

  async function submit(event) {
    event.preventDefault();
    const content = form.content.trim();
    if (!content) {
      setError("请输入内容");
      return;
    }
    if (!form.date || !form.time) {
      setError("请选择日期和时间");
      return;
    }
    if (form.repeat === "自定义" && customDates.length === 0) {
      setError("请至少添加一个自定义日期");
      return;
    }

    const dates = [...customDates].sort();
    const startsOn = form.repeat === "自定义" ? dates[0] : form.date;
    const payload = {
      content,
      starts_at: dateTimeAt(startsOn, form.time),
      repeat_mode: REPEAT_VALUES[form.repeat],
      notify_mode: REMINDER_VALUES[form.reminder],
      ...(form.color ? { color: form.color } : {}),
      ...(form.repeat === "自定义" ? { custom_dates: dates } : {}),
    };

    setSaving(true);
    setError("");
    try {
      const created = await createTodo(payload);
      await notifyDataChanged({ type: "created", todoId: created.id });
      closeCurrentWindow();
    } catch (createError) {
      setError(createError.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <WindowFrame title="新日程" className="is-form-window">
      <form className="side-form utility-form" onSubmit={submit}>
        <label>
          <span>内容</span>
          <textarea
            value={form.content}
            rows={5}
            maxLength={500}
            autoFocus
            onChange={(event) => setField("content", event.target.value)}
          />
        </label>

        <SelectField
          label="重复"
          value={form.repeat}
          options={Object.keys(REPEAT_VALUES)}
          onChange={setRepeat}
        />

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

        <SelectField
          label="提醒"
          value={form.reminder}
          options={Object.keys(REMINDER_VALUES)}
          onChange={(value) => setField("reminder", value)}
        />

        <ColorField value={form.color} onChange={(value) => setField("color", value)} allowRandom />

        {error && <p className="form-error" role="alert">{error}</p>}
        <footer className="side-footer utility-footer">
          <button className="cancel-button" type="button" onClick={closeCurrentWindow}>取消</button>
          <button className="save-button" type="submit" disabled={saving}>{saving ? "创建中" : "创建"}</button>
        </footer>
      </form>
    </WindowFrame>
  );
}

function LoadingWindow({ title, message = "正在读取…" }) {
  return (
    <WindowFrame title={title}>
      <div className="utility-state" role="status"><span />{message}</div>
    </WindowFrame>
  );
}

function DetailWindow() {
  const todoID = Number(params.get("todo_id"));
  const occurrenceDate = validDate(params.get("date"));
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

  async function load() {
    const [todo, calendar] = await Promise.all([
      getTodo(todoID),
      getCalendar(occurrenceDate, nextDateKey(occurrenceDate)),
    ]);
    const occurrence = calendar.data.find((item) => item.todo_id === todoID);
    const startsAt = shanghaiDateTimeParts(todo.starts_at);
    const nextRecord = {
      todo,
      occurrenceDone: Boolean(occurrence?.occurrence_done),
      allDone: Boolean(todo.all_done),
    };
    setRecord(nextRecord);
    setForm({
      content: todo.content,
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
      todo: {
        ...current.todo,
        content: result.content,
        version: result.version,
      },
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
    const editorWindow = window.open(
      editorURL,
      `note-content-editor-${todoID}`,
      "popup,width=760,height=560,resizable=yes",
    );
    editorWindow?.focus();
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
    const content = form.content.trim();
    if (!content) {
      setError("请输入内容");
      return;
    }
    if (form.repeat === "自定义" && customDates.length === 0) {
      setError("请至少添加一个自定义日期");
      return;
    }

    const startsOn = form.repeat === "自定义" ? [...customDates].sort()[0] : form.date;
    const payload = {
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
      closeCurrentWindow();
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
      closeCurrentWindow();
    } catch (deleteError) {
      setDeleteConfirming(false);
      setError(deleteError.message);
    } finally {
      setDeleting(false);
    }
  }

  if (loading) return <LoadingWindow title="日程详情" />;
  if (!record || !form) return <LoadingWindow title="日程详情" message={error || "没有找到这条日程"} />;

  return (
    <WindowFrame title="日程详情" subtitle={preciseDateLabel(occurrenceDate)} className="is-form-window">
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

        <div className="content-editor-field">
          <label htmlFor="detail-content">内容</label>
          <div className="content-editor-field-shell">
            <textarea id="detail-content" value={form.content} rows={5} maxLength={500} autoFocus onChange={(event) => setField("content", event.target.value)} />
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
          <button className="cancel-button" type="button" onClick={closeCurrentWindow}>关闭</button>
          <button className="save-button" type="submit" disabled={saving || deleting || Boolean(completionSaving)}>{saving ? "保存中" : "保存"}</button>
        </footer>
      </form>
    </WindowFrame>
  );
}

function SettingsWindow() {
  return (
    <WindowFrame title="外观设置" subtitle="按自己的工作环境调整 Note" className="is-settings-window">
      <AppearanceSettingsForm onDone={closeCurrentWindow} />
    </WindowFrame>
  );
}

function ContentEditorWindow() {
  const [editorState, setEditorState] = useState(null);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const fallbackTodoID = Number(params.get("todo_id"));
    const request = window.noteDesktop?.getContentEditorState?.()
      || (Number.isSafeInteger(fallbackTodoID) && fallbackTodoID > 0
        ? getTodo(fallbackTodoID).then((todo) => ({
            todoId: todo.id,
            date: validDate(params.get("date")),
            content: todo.content,
            version: todo.version,
          }))
        : null);
    if (!request) {
      setError("专注编辑窗口仅在桌面版中可用");
      setLoading(false);
      return;
    }
    request
      .then((state) => {
        if (!state) throw new Error("编辑会话已经结束");
        setEditorState(state);
        setContent(state.content || "");
      })
      .catch((loadError) => setError(loadError.message))
      .finally(() => setLoading(false));
  }, []);

  async function saveAndReturn(event) {
    event.preventDefault();
    const nextContent = content.trim();
    if (!nextContent) {
      setError("内容不能为空");
      return;
    }
    if (!editorState) return;

    setSaving(true);
    setError("");
    try {
      const updated = await patchTodo(editorState.todoId, {
        content: nextContent,
        version: editorState.version,
      });
      await notifyDataChanged({ type: "updated", todoId: editorState.todoId });
      const result = {
        todoId: editorState.todoId,
        content: updated.content,
        version: updated.version,
      };
      if (window.noteDesktop?.finishContentEditor) {
        await window.noteDesktop.finishContentEditor(result);
      } else {
        window.opener?.postMessage({ type: "note:content-editor-saved", payload: result }, window.location.origin);
        closeCurrentWindow();
      }
    } catch (saveError) {
      setError(saveError.message);
      setSaving(false);
    }
  }

  if (loading) return <LoadingWindow title="专注编辑" />;
  if (!editorState) return <LoadingWindow title="专注编辑" message={error || "编辑会话已经结束"} />;

  return (
    <WindowFrame title="专注编辑" subtitle="Ctrl + Enter 保存并返回" className="is-content-editor-window">
      <form className="focused-editor-form" onSubmit={saveAndReturn}>
        <label htmlFor="focused-content">日程内容</label>
        <textarea
          id="focused-content"
          value={content}
          maxLength={500}
          autoFocus
          disabled={saving}
          onChange={(event) => {
            setContent(event.target.value);
            setError("");
          }}
          onKeyDown={(event) => {
            if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
              event.preventDefault();
              event.currentTarget.form?.requestSubmit();
            }
          }}
        />
        <div className="focused-editor-status">
          <span className={error ? "is-error" : ""} role={error ? "alert" : undefined}>
            {error || `${content.length} / 500`}
          </span>
          <button className="focused-editor-save" type="submit" disabled={saving}>
            <CheckCircle size={21} weight="fill" aria-hidden="true" />
            {saving ? "保存中" : "保存并返回"}
          </button>
        </div>
      </form>
    </WindowFrame>
  );
}

function ReminderWindow() {
  const [reminder, setReminder] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const detailButtonRef = useRef(null);

  useEffect(() => {
    const fallbackTodoID = Number(params.get("todo_id"));
    const fallbackOccursAt = params.get("occurs_at");
    const fallback = Number.isSafeInteger(fallbackTodoID)
      && fallbackTodoID > 0
      && !Number.isNaN(Date.parse(fallbackOccursAt))
      ? {
          todoId: fallbackTodoID,
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
    <WindowFrame
      title="提醒"
      className="is-reminder-window"
      closeLabel="关闭提醒"
    >
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
            <p>{reminder.content}</p>
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

function DayWindow() {
  const [date, setDate] = useState(() => validDate(params.get("date")));
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

  useEffect(() => {
    if (!window.noteDesktop?.onDayDateChanged) return undefined;
    return window.noteDesktop.onDayDateChanged((payload) => {
      const nextDate = validDate(payload?.date);
      const nextURL = new URL(window.location.href);
      nextURL.searchParams.set("date", nextDate);
      window.history.replaceState(null, "", nextURL);
      document.title = `${nextDate} · Note`;
      setDate(nextDate);
    });
  }, []);

  async function saveItem(item, changes) {
    const latest = events.find((eventItem) => eventItem.todoId === item.todoId) || item;
    const payload = { version: latest.version };
    if (changes.content !== undefined) payload.content = changes.content;
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

  function openDetails(item) {
    window.noteDesktop?.openDetail?.({ todoId: item.todoId, date: item.date });
  }

  function openComposer() {
    const request = window.noteDesktop?.openCompose?.({ date });
    request?.catch((openError) => setError(openError.message));
  }

  function openCalendar() {
    const request = window.noteDesktop?.openCalendar?.();
    request?.catch((openError) => setError(openError.message));
  }

  function openSettings() {
    const request = window.noteDesktop?.openSettings?.();
    if (request) {
      request.catch((openError) => setError(openError.message));
      return;
    }
    const settingsURL = new URL(window.location.href);
    settingsURL.search = "";
    settingsURL.searchParams.set("window", "settings");
    window.location.assign(settingsURL);
  }

  const content = useMemo(() => {
    if (loading) return [];
    return events;
  }, [events, loading]);

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
        onAdd={openComposer}
        onOpenCalendar={openCalendar}
        onOpenSettings={openSettings}
        onSave={saveItem}
        onOpenDetails={openDetails}
        onComplete={completeItem}
        onDelete={removeItem}
        primaryWindow
        windowControls
      />
      {error && <div className="utility-day-error" role="alert">{error}</div>}
    </main>
  );
}

export default function StandaloneWindow({ role }) {
  if (role === "create") return <CreateWindow />;
  if (role === "detail") return <DetailWindow />;
  if (role === "reminder") return <ReminderWindow />;
  if (role === "day") return <DayWindow />;
  if (role === "settings") return <SettingsWindow />;
  if (role === "content-editor") return <ContentEditorWindow />;
  return <LoadingWindow title="Note" message="未知窗口" />;
}
