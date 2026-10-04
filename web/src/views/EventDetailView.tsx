import type { CalendarEvent } from "../types";
import { useEffect, useState } from "react";
import { getEvent, patchEvent, deleteEvent } from "../api";
import { REPEAT_LABELS, shanghaiDateTimeParts } from "../lib/calendar";
import { auth } from "../auth";
import { onDataChanged } from "../data-events";
import Avatar from "../components/Avatar";
import SchedulePermissions from "../components/SchedulePermissions";
import { useGroups } from "../groups";
import WindowFrame, { LoadingWindow } from "../windows/WindowFrame";
import { closeCurrentWindow, notifyDataChanged } from "../windows/window-utils";

const inputTime = (value: string) => {
  const p = shanghaiDateTimeParts(value);
  return `${p.date}T${p.time}`;
};
export default function EventDetailView({ eventId }: { eventId: number }) {
  const { groups } = useGroups();
  const [record, setRecord] = useState<CalendarEvent | null>(null),
    [form, setForm] = useState<{title: string; content: string; start: string; end: string} | null>(null),
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
        if (active) setError(e instanceof Error ? e.message : "操作失败");
      });
    return () => {
      active = false;
    };
  }, [eventId]);
  useEffect(() => {
    let active = true, generation = 0;
    const remove = onDataChanged(change => {
      if (!["permissions", "groups", "profile"].includes(change.type)) return;
      const current = ++generation;
      void getEvent(eventId).then(item => {
        if (active && generation === current) setRecord(record => record ? { ...record, my_role: item.my_role, member_roles: item.member_roles, creator: item.creator } : record);
      }).catch(reason => {
        if (!active || generation !== current) return;
        setError(reason instanceof Error ? reason.message : "读取权限失败");
        if (reason?.status === 403 || reason?.status === 404) setRecord(null);
      });
    });
    return () => { active = false; remove(); };
  }, [eventId]);
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!form || !record || record.my_role !== "editor") return;
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
      setError(e instanceof Error ? e.message : "操作失败");
    } finally {
      setSaving(false);
    }
  }
  if (!form || !record)
    return <LoadingWindow title="日程详情" message={error || "正在读取…"} />;
  const canEdit = record.my_role === "editor";
  const creator = record.creator_id === auth.user?.id;
  return (
    <WindowFrame title="日程详情" className="is-form-window">
      <form className="side-form utility-form" onSubmit={save}>
        <div className="group-context"><span>{groups.find(group => group.id === record.group_id)?.name || "群组"}</span>{record.creator && <><Avatar url={record.creator.avatar} name={record.creator.nickname || record.creator.username} size={24} /><span>{record.creator.nickname || record.creator.username} 创建</span></>}<small>{canEdit ? "可编辑" : "只读"}</small></div>
        <fieldset className="schedule-fields" disabled={!canEdit || saving}>
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
        </fieldset>
        {creator && <SchedulePermissions kind="event" id={eventId} groupID={record.group_id} creatorID={record.creator_id} roles={record.member_roles} onChanged={async () => { const item = await getEvent(eventId); setRecord(record => record ? { ...record, my_role: item.my_role, member_roles: item.member_roles } : record); }} />}
        {error && <div role="alert">{error}</div>}
        {creator && <button type="button" className="delete-button" disabled={saving} onClick={() => { if (!window.confirm("确认删除这条日程及整个重复周期吗？")) return; setSaving(true); setError(""); void deleteEvent(eventId).then(async () => { await notifyDataChanged({ type: "deleted", eventId }); closeCurrentWindow(); }).catch(reason => setError(reason instanceof Error ? reason.message : "删除失败")).finally(() => setSaving(false)); }}>删除日程</button>}
        {canEdit && <button type="submit" className="save-button event-save-button" disabled={saving}>
          {saving ? "保存中" : "保存"}
        </button>}
      </form>
    </WindowFrame>
  );
}
