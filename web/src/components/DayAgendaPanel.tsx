import { useState } from "react";
import { Check, CaretRight } from "@phosphor-icons/react";
import { patchOccurrence } from "../api";
import type { CalendarItem } from "../types";
import { occursOnDay } from "../lib/timeline";

// Adapted from b0d3205's DayAgendaPanel: retain compact cards and completion
// grouping, with a separate event group and one shared detail panel.
export default function DayAgendaPanel({ date, items, onOpen, onRefresh }: {
  date: string; items: CalendarItem[]; onOpen: (item: CalendarItem) => void;
  onRefresh: () => Promise<void>;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const today = items.filter(item => occursOnDay(item, date)).sort((a, b) => a.time.localeCompare(b.time));
  const done = (item: CalendarItem) => Boolean(item.allDone || item.occurrenceDone);
  const groups = [
    { label: "未完成", items: today.filter(item => item.kind === "todo" && !done(item)) },
    { label: "日程", items: today.filter(item => item.kind === "event") },
    { label: "已完成", items: today.filter(item => item.kind === "todo" && done(item)) },
  ];
  async function complete(item: CalendarItem) {
    if (!item.todoId || busy) return;
    setBusy(item.id); setError("");
    try { await patchOccurrence(item.todoId, item.date, !done(item)); await onRefresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "更新失败"); }
    finally { setBusy(null); }
  }
  return <section className="day-tags" aria-label={`${date}标签日程`}>
    {error && <p role="alert">{error}</p>}
    {!today.length && <p className="day-tags-empty">这一天暂无日程</p>}
    {groups.filter(group => group.items.length).map(group => <section key={group.label}>
      <h3>{group.label}<small>{group.items.length}</small></h3>
      {group.items.map(item => <article className={`day-tag ${done(item) ? "is-done" : ""}`} key={item.id} data-search-key={`${item.kind}-${item.eventId || item.todoId}`}>
        {item.kind === "todo" ? <button className="tag-completion" style={{ color: item.color }} disabled={Boolean(busy) || item.allDone} aria-label={`${done(item) ? "取消" : "标记"}${item.title}当天完成`} onClick={() => void complete(item)}>{done(item) ? <Check size={16} /> : "○"}</button> : <span style={{ color: item.color }}>▰</span>}
        <button className="tag-detail" onClick={() => onOpen(item)}><strong>{item.title}</strong><time>{item.time}{item.kind === "event" ? ` – ${item.endTime}` : ""}</time><CaretRight size={14} /></button>
      </article>)}
    </section>)}
  </section>;
}
