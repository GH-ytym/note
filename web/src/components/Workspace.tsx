import { createContext, useEffect, useState, useRef } from "react";
import type { ReactNode } from "react";
import useNotifications from "../hooks/useNotifications";
import type { NotificationCard } from "../hooks/useNotifications";
import NotificationMenu from "./NotificationMenu";
import Avatar from "./Avatar";
import useWindowDrag from "../hooks/useWindowDrag";
import WindowControls from "./WindowControls";
import CalendarWindow from "../views/CalendarWindow";
import SettingsView from "../views/SettingsView";

import DetailView from "../views/DetailView";
import EventDetailView from "../views/EventDetailView";
import "../styles/workspace.css";

type Page = { id: string; title: string; card?: NotificationCard; date?: string; todoId?: number; eventId?: number };
export const InboxContext = createContext<ReturnType<typeof useNotifications> | null>(null);
export function openPage(page: Page) { window.dispatchEvent(new CustomEvent("note-open-page", {detail: page})); }
export default function Workspace() {
  const inbox = useNotifications();
  const drag = useWindowDrag();
  const [tabs, setTabs] = useState<Page[]>([{id: "calendar", title: "N · 日历"}]);
  const dragId = useRef<string | null>(null);
  const strip = useRef<HTMLElement>(null);
  const [picking, setPicking] = useState(false);
  const [mini, setMini] = useState(false);
  const [active, setActive] = useState("calendar");
  useEffect(() => {
    const open = (event: Event) => { const page = (event as CustomEvent<Page>).detail; setTabs(old => old.some(tab => tab.id === page.id) ? old : [...old, page]); setActive(page.id); };
    window.addEventListener("note-open-page", open);
    return () => window.removeEventListener("note-open-page", open);
  }, []);
  useEffect(() => {
    strip.current?.querySelector<HTMLElement>('[aria-current="page"]')?.scrollIntoView({block: "nearest", inline: "nearest"});
  }, [active, tabs.length]);
  function move(from: string, to: string) {
    setTabs(old => {
      const next = [...old];
      const a = next.findIndex(tab => tab.id === from), b = next.findIndex(tab => tab.id === to);
      if (a < 0 || b < 0 || a === b) return old;
      next.splice(b, 0, ...next.splice(a, 1));
      return next;
    });
  }
  function close(id: string) { setTabs(old => old.filter(tab => tab.id !== id)); if (active === id) setActive("calendar"); }
  function content(tab: Page): ReactNode {
    const done = () => close(tab.id);
    if (tab.id === "notifications") return <><h1>通知</h1><NotificationMenu page /></>;
    if (tab.id === "settings") return <SettingsView onDone={done} />;

    if (tab.eventId) return <EventDetailView eventId={tab.eventId} onDone={done} />;
    if (tab.todoId) return <DetailView todoId={tab.todoId} date={tab.date} onDone={done} />;
    const card = inbox.items.find(item => item.id === tab.card?.id) ?? tab.card;
    if (!card) return null;
    return <section className="notification-detail"><small>通知详情</small><h1>{card.type === "join_requested" ? "入群申请" : "群组通知"}</h1><Avatar url={card.actor_avatar} name={card.actor_name} /><h2>{card.actor_name}</h2><p>{card.type === "join_requested" ? "申请加入" : "加入了"}「{card.group_name}」</p><time>{new Date(card.created_at).toLocaleString("zh-CN")}</time>{card.request_status === "pending" && <p title="审核功能尚未开放"><button disabled>同意</button> <button disabled>拒绝</button><small>审核功能尚未开放</small></p>}<button onClick={() => openPage({id: "notifications", title: "通知"})}>查看全部通知</button></section>;
  }
  return <InboxContext.Provider value={inbox}><div className="workspace"><header className="workspace-topbar" hidden={mini} {...drag}><nav inert={picking} ref={strip} className="workspace-tabs" aria-label="页面标签" onWheel={event => {
    if (Math.abs(event.deltaY) > Math.abs(event.deltaX)) event.currentTarget.scrollLeft += event.deltaY;
  }}>{tabs.map(tab => <div key={tab.id} draggable className={active === tab.id ? "active" : ""}
    onDragStart={event => { dragId.current = tab.id; event.dataTransfer.effectAllowed = "move"; event.dataTransfer.setData("text/plain", tab.id); }}
    onDragEnd={() => { dragId.current = null; }}
    onDragOver={event => {
      if (!dragId.current) return;
      event.preventDefault();
      const box = event.currentTarget.parentElement!.getBoundingClientRect();
      if (event.clientX < box.left + 35) strip.current!.scrollLeft -= 18;
      if (event.clientX > box.right - 35) strip.current!.scrollLeft += 18;
    }}
    onDrop={event => { event.preventDefault(); if (dragId.current) move(dragId.current, tab.id); dragId.current = null; }}>
    <button className="tab-title" title={tab.title} aria-current={active === tab.id ? "page" : undefined} onClick={() => setActive(tab.id)}
      onKeyDown={event => { if (event.altKey && (event.key === "ArrowLeft" || event.key === "ArrowRight")) { event.preventDefault(); const i = tabs.findIndex(t => t.id === tab.id); const target = tabs[i + (event.key === "ArrowLeft" ? -1 : 1)]; if (target) move(tab.id, target.id); } }}>{tab.title}</button>
    {tab.id !== "calendar" && <button className="tab-close" aria-label={`关闭${tab.title}`} onClick={() => close(tab.id)}>×</button>}
  </div>)}</nav><WindowControls variant="hide-app" /></header><div className="workspace-calendar" hidden={!mini && active !== "calendar"}><CalendarWindow onPickingChange={setPicking} onMiniChange={setMini} /></div>{tabs.filter(tab => tab.id !== "calendar").map(tab => <section key={tab.id} hidden={mini || active !== tab.id} className="workspace-page">{content(tab)}</section>)}</div></InboxContext.Provider>;
}
