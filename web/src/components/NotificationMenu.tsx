import { useEffect, useRef, useState } from "react";
import { Bell, CaretUp, ArrowClockwise, X } from "@phosphor-icons/react";
import useNotifications from "../hooks/useNotifications";
import Avatar from "./Avatar";
import "../styles/notifications.css";

const statusLabels: Record<string, string> = {
  pending: "待审核", accepted: "已同意", rejected: "已拒绝", cancelled: "已撤销",
};

export default function NotificationMenu({ collapsed }: { collapsed: boolean }) {
  const { items, unread_count, loading, error, connected, reload } = useNotifications();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const close = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    close.current?.focus();
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setOpen(false); trigger.current?.focus(); }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);
  return <div className="notification-anchor" ref={root}>
    <button ref={trigger} className="notification-trigger" aria-label={`通知，${unread_count} 条未读`} aria-expanded={open} aria-controls="notification-panel" onClick={() => { setOpen(!open); if (!open) reload(); }} title="通知">
      <Bell size={18} /><span className={collapsed ? "notification-sr-only" : "notification-trigger-label"}>通知</span>
      {unread_count > 0 && <span className="notification-count">{unread_count > 99 ? "99+" : unread_count}</span>}
      {!collapsed && <CaretUp size={14} className={open ? "is-open" : ""} />}
    </button>
    {open && <section id="notification-panel" className="notification-panel" role="region" aria-label="通知列表">
      <header><div><strong>通知</strong><span>{unread_count} 条未读</span></div><button aria-label="刷新通知" title="刷新通知" disabled={loading} onClick={reload}><ArrowClockwise size={17} /></button><button ref={close} aria-label="收起通知" onClick={() => { setOpen(false); trigger.current?.focus(); }}><X size={17} /></button></header>
      <div className="notification-connection" role="status"><i className={connected ? "is-connected" : ""} />{connected ? "实时接收中" : "实时连接重试中 · 列表每分钟同步"}</div>
      {error && <p className="notification-error" role="alert">{error}<button onClick={reload}>重试</button></p>}
      <div className="notification-list" aria-busy={loading}>
        {!items.length && <div className="notification-empty"><Bell size={28} /><p>{loading ? "正在加载通知…" : error ? "暂时无法读取通知" : "暂无通知"}</p><small>入群提醒和申请会显示在这里</small></div>}
        {items.map(item => <article className={`notification-item ${item.read_at ? "" : "is-unread"}`} key={item.id}>
          <Avatar url={item.actor_avatar} name={item.actor_name} />
          <div><p><strong>{item.actor_name}</strong>{item.type === "join_requested" ? " 申请加入 " : item.type === "joined" ? " 加入了 " : " 更新了 "}<strong>{item.group_name}</strong></p>
            <div className="notification-meta"><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}</time>{item.request_status && <span>{statusLabels[item.request_status] || item.request_status}</span>}{!item.read_at && <span>未读</span>}</div>
          </div>
        </article>)}
      </div>
      <footer>最近 50 条通知 · 已读和审批操作暂未开放</footer>
    </section>}
  </div>;
}
