import { useEffect, useRef, useState, useContext } from "react";
import { Bell, CaretUp } from "@phosphor-icons/react";
import type { NotificationCard } from "../hooks/useNotifications";
import { InboxContext, openPage } from "./Workspace";
import SpotlightCard from "./react-bits/SpotlightCard";
import Avatar from "./Avatar";
import "../styles/notifications.css";

const statusLabels: Record<string, string> = {
  pending: "待审核", accepted: "已同意", rejected: "已拒绝", cancelled: "已撤销",
};

function NotificationContent({ collapsed = true, page = false }: { collapsed?: boolean; page?: boolean }) {
  const { items, unread_count, loading, error, connected, reload, next, loadingMore, loadMore, markRead } = useContext(InboxContext)!;
  const [frozen, setFrozen] = useState<NotificationCard[] | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [saving, setSaving] = useState(false);
  const [actionError, setActionError] = useState("");
  const visible = frozen ?? (page ? items : items.filter(item => !item.read_at).slice(0, 3));
  const selectable = (frozen ?? []).filter(item => !item.read_at);
  function finishSelection() { setFrozen(null); setSelected(new Set()); setActionError(""); }
  async function submit() {
    setSaving(true); setActionError("");
    try { await markRead([...selected]); finishSelection(); }
    catch(reason) { setActionError(reason instanceof Error ? reason.message : "标记失败，请重试"); }
    finally { setSaving(false); }
  }
  const [open, setOpen] = useState(false);
  useEffect(() => {
    if (open && !page && connected && !loadingMore && next && items.filter(item => !item.read_at).length < Math.min(3, unread_count)) void loadMore();
  }, [open, page, connected, loadingMore, next, items, unread_count, loadMore]);
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
  return <div className={page ? "notification-page" : "notification-anchor"} ref={root}>
    {!page && <button ref={trigger} className="notification-trigger" aria-label={`通知，${unread_count} 条未读`} aria-expanded={open} aria-controls="notification-preview" onClick={() => { setOpen(!open); }} title="通知">
      <Bell size={18} /><span className={collapsed ? "notification-sr-only" : "notification-trigger-label"}>通知</span>
      {unread_count > 0 && <span className="notification-count">{unread_count > 99 ? "99+" : unread_count}</span>}
      {!collapsed && <CaretUp size={14} className={open ? "is-open" : ""} />}
    </button>}
    {(page || open) && <section id={page ? "notification-page-list" : "notification-preview"} className="notification-panel" role="region" aria-label="通知列表">
      {page && (frozen ? <div className="notification-selection">
        <label><input type="checkbox" disabled={saving || !selectable.length} checked={!!selectable.length && selected.size === selectable.length}
          onChange={e => setSelected(new Set(e.target.checked ? selectable.map(item => item.id) : []))} />全选当前列表</label>
        <span>已选 {selected.size} 条</span>
        <button disabled={saving || !selected.size} onClick={() => void submit()}>{saving ? "处理中…" : "标为已读"}</button>
        <button disabled={saving} onClick={finishSelection}>取消</button>
      </div> : <div className="notification-selection"><button disabled={loading || !connected || !items.some(item => !item.read_at)}
        onClick={() => { setFrozen(items.map(item => ({...item}))); setSelected(new Set()); }}>批量已读</button></div>)}
      {actionError && <p className="notification-error" role="alert">{actionError}</p>}
      {error && <p className="notification-error" role="alert">{error}<button onClick={reload}>重试</button></p>}
      <div className="notification-list" aria-busy={loading}>
        {!visible.length && <div className="notification-empty"><Bell size={28} /><p>{loading ? "正在加载通知…" : error ? "暂时无法读取通知" : "暂无通知"}</p><small>入群提醒和申请会显示在这里</small></div>}
        {visible.map(item => <SpotlightCard theme="light" spotlightColor="var(--yellow)" intensity={0.18} proximity={0} className={`notification-item ${item.read_at ? "" : "is-unread"}`} key={item.id}>
          {frozen && <input type="checkbox" aria-label={`选择通知：${item.actor_name} ${item.group_name}`} disabled={saving || !!item.read_at}
            checked={selected.has(item.id)} onChange={e => { const ids = new Set(selected); if(e.target.checked) ids.add(item.id); else ids.delete(item.id); setSelected(ids); }} />}
          <Avatar url={item.actor_avatar} name={item.actor_name} />
          <div><button className="notification-card-link" onClick={() => { openPage({id: `notification-${item.id}`, title: "通知详情", card: item}); setOpen(false); }}><p><strong>{item.actor_name}</strong>{item.type === "join_requested" ? " 申请加入 " : item.type === "joined" ? " 加入了 " : " 更新了 "}<strong>{item.group_name}</strong></p></button>
            <div className="notification-meta"><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}</time>{item.request_status && <span>{statusLabels[item.request_status] || item.request_status}</span>}{!item.read_at && <span>未读</span>}</div>
            {item.type === "join_requested" && item.request_status === "pending" && <div className="notification-actions" title="审核功能尚未开放"><button disabled>同意</button><button disabled>拒绝</button></div>}
          </div>
        </SpotlightCard>)}
      </div>
      <footer>{!page ? <button onClick={() => { openPage({id: "notifications", title: "通知"}); setOpen(false); }}>更多...</button> : frozen ? "多选期间列表已固定，新通知将在退出后显示" :
        next ? <button disabled={loadingMore || !connected} onClick={() => void loadMore()}>{loadingMore ? "加载中…" : "加载更多"}</button> : "已显示全部通知"}</footer>
    </section>}
  </div>;
}

export default function NotificationMenu(props: { collapsed?: boolean; page?: boolean }) {
  const inbox = useContext(InboxContext);
  return inbox ? <NotificationContent {...props} /> : null;
}
