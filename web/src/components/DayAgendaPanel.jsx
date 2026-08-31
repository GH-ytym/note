import { useEffect, useState } from "react";
import {
  CalendarBlank,
  Check,
  GearSix,
  Plus,
  Trash,
  X,
} from "@phosphor-icons/react";
import { colorWithAlpha, isEventDone, preciseDateLabel } from "../lib/calendar";
import { TimeField } from "./FormFields";
import WindowControls from "./WindowControls";

function DayAgendaItem({ item, onSave, onOpenDetails, onComplete, onDelete }) {
  const [title, setTitle] = useState(item.title);
  const [time, setTime] = useState(item.time);
  const [saving, setSaving] = useState(false);
  const [action, setAction] = useState(null);
  const [error, setError] = useState("");
  const done = isEventDone(item);
  const busy = saving || action !== null;

  useEffect(() => {
    setTitle(item.title);
    setTime(item.time);
  }, [item.title, item.time]);

  async function commit(changes) {
    setSaving(true);
    setError("");
    try {
      await onSave(item, changes);
    } catch (saveError) {
      setError(saveError.message);
      setTitle(item.title);
      setTime(item.time);
    } finally {
      setSaving(false);
    }
  }

  function commitTitle() {
    const nextTitle = title.trim();
    if (!nextTitle) {
      setError("标题不能为空");
      setTitle(item.title);
      return;
    }
    if (nextTitle !== item.title) void commit({ title: nextTitle });
  }

  function commitTime(nextTime) {
    setTime(nextTime);
    if (nextTime !== item.time) void commit({ time: nextTime });
  }

  async function completeForDay() {
    if (done || !onComplete || busy) return;
    setAction("complete");
    setError("");
    try {
      await onComplete(item);
    } catch (completeError) {
      setError(completeError.message);
    } finally {
      setAction(null);
    }
  }

  async function removeItem() {
    if (!onDelete || busy) return;
    setAction("delete");
    setError("");
    try {
      await onDelete(item);
    } catch (deleteError) {
      setError(deleteError.message);
    } finally {
      setAction(null);
    }
  }

  return (
    <article
      className={`agenda-item ${done ? "is-complete" : "is-pending"} ${busy ? "is-saving" : ""} ${onDelete ? "has-delete" : ""}`}
      style={{
        "--agenda-color": item.color,
        "--agenda-soft": colorWithAlpha(item.color, 0.18),
      }}
    >
      <button
        className={`agenda-dot-button ${action === "complete" ? "is-confirming" : ""}`}
        type="button"
        disabled={done || !onComplete || busy}
        onClick={completeForDay}
        aria-label={done ? `${item.title}当天已完成` : `标记${item.title}为当天完成`}
      >
        {action === "complete"
          ? <Check className="agenda-complete-check" size={13} weight="bold" aria-hidden="true" />
          : <span className="agenda-dot" aria-hidden="true" />}
      </button>
      <input
        className="agenda-title"
        value={title}
        maxLength={50}
        disabled={busy}
        aria-label={`${item.title}的标题`}
        onChange={(event) => setTitle(event.target.value)}
        onBlur={commitTitle}
        onKeyDown={(event) => {
          if (event.key === "Enter") event.currentTarget.blur();
          if (event.key === "Escape") {
            setTitle(item.title);
            event.currentTarget.blur();
          }
        }}
      />
      <button
        className="agenda-detail-chevron"
        type="button"
        disabled={busy}
        onClick={() => onOpenDetails(item)}
        aria-label={`进入${item.title}的详情`}
      >
        <span aria-hidden="true">&gt;</span>
      </button>
      <TimeField compact label={`${item.title}的时间`} value={time} onChange={commitTime} />
      {onDelete && (
        <button
          className="agenda-delete-button"
          type="button"
          disabled={busy}
          onClick={removeItem}
          aria-label={`删除${item.title}`}
          title="删除日程"
        >
          <Trash size={16} weight="regular" aria-hidden="true" />
        </button>
      )}
      {error && <small className="agenda-error">{error}</small>}
    </article>
  );
}

export default function DayAgendaPanel({
  dateKey,
  items,
  onAdd,
  onClear,
  onSave,
  onToggleCalendar,
  calendarOpen = false,
  onOpenDetails,
  onOpenSettings,
  onComplete,
  onDelete,
  workspaceWindow = false,
  windowControls = false,
}) {
  const groups = [
    { key: "pending", label: "未完成", items: items.filter((item) => !isEventDone(item)) },
    { key: "complete", label: "已完成", items: items.filter(isEventDone) },
  ].filter((group) => group.items.length > 0);

  return (
    <section
      className={`day-agenda-panel ${dateKey ? "has-selection" : ""}`}
      aria-label={dateKey ? `${preciseDateLabel(dateKey)}的日程` : "日期日程"}
    >
      <header className="day-agenda-panel-header">
        <div className="day-agenda-panel-title">
          <span className="panel-accent" aria-hidden="true" />
          <div>
            <strong>{dateKey ? preciseDateLabel(dateKey) : "日程"}</strong>
            {onToggleCalendar && (
              <button
                className="day-agenda-calendar-button"
                type="button"
                onClick={onToggleCalendar}
                aria-label={calendarOpen ? "收回日历" : "展开日历"}
                aria-expanded={calendarOpen}
                title={calendarOpen ? "收回日历" : "展开日历"}
              >
                <CalendarBlank size={17} weight={calendarOpen ? "fill" : "regular"} aria-hidden="true" />
              </button>
            )}
            <small>{dateKey ? `${items.length} 条` : "选择一个日期"}</small>
            {onAdd && (
              <button className="day-agenda-inline-add" type="button" onClick={onAdd} aria-label="添加日程">
                <Plus size={18} weight="regular" aria-hidden="true" />
              </button>
            )}
          </div>
        </div>

        <div className="day-agenda-header-actions">
          {onOpenSettings && (
            <button className="day-agenda-settings" type="button" onClick={onOpenSettings} aria-label="打开外观设置" title="外观设置">
              <GearSix size={18} weight="regular" aria-hidden="true" />
            </button>
          )}
          {windowControls ? (
            <WindowControls variant={workspaceWindow ? "hide-app" : "close"} />
          ) : dateKey && (
            <button type="button" onClick={onClear} aria-label="清空选中日期">
              <X size={18} />
            </button>
          )}
        </div>
      </header>

      <div className="day-agenda-panel-body">
        {dateKey && items.length > 0 ? (
          <div className="day-agenda-list">
            {groups.map((group) => (
              <section className={`agenda-section is-${group.key}`} key={group.key} aria-label={group.label}>
                <header className="agenda-section-heading">
                  <span>{group.label}</span>
                  <small>{group.items.length}</small>
                </header>
                <div className="agenda-section-items">
                  {group.items.map((item) => (
                    <DayAgendaItem
                      item={item}
                      onSave={onSave}
                      onOpenDetails={onOpenDetails}
                      onComplete={onComplete}
                      onDelete={onDelete}
                      key={item.id}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        ) : (
          <div className="day-agenda-panel-empty" aria-hidden="true">
            <span />
          </div>
        )}
      </div>
    </section>
  );
}
