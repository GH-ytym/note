import { useEffect, useRef, useState } from "react";
import { Plus, Users, GearSix, CaretLeft, CaretRight, SignIn, X } from "@phosphor-icons/react";
import { QRCodeSVG } from "qrcode.react";
import { auth } from "../auth";
import { useGroups } from "../groups";
import { useProfile } from "../profile";
import type { Group, GroupMember } from "../types";
import { createGroup, joinGroup, groupMembers, groupInvite, refreshGroupInvite, quitGroup, dismissGroup } from "../api";
import { publishDataChanged } from "../data-events";
import { groupColor, groupInitial, parseInvite, invitationValue } from "../lib/groups";
import Avatar from "./Avatar";
import "../styles/groups.css";

export function GroupIcon({ group }: { group: Pick<Group, "id" | "name"> }) {
  return <span className="group-icon" style={{ backgroundColor: groupColor(group.id) }} aria-hidden="true">{groupInitial(group.name)}</span>;
}

export default function GroupSidebar({ onSettings }: { onSettings: () => void }) {
  const { groups, selected, select, loading, error, reload } = useGroups();
  const { profile } = useProfile();
  const [collapsed, setCollapsed] = useState(false);
  const [panel, setPanel] = useState<"create" | "join" | "manage" | null>(() => parseInvite(window.location.href) ? "join" : null);
  return <aside className={`group-sidebar ${collapsed ? "is-collapsed" : ""}`} aria-label="群组侧边栏">
    <header className="group-sidebar-header"><strong><span className="note-mark">N</span>{!collapsed && "Note"}</strong><button title={collapsed ? "展开侧边栏" : "收起侧边栏"} aria-label={collapsed ? "展开侧边栏" : "收起侧边栏"} onClick={() => setCollapsed(!collapsed)}>{collapsed ? <CaretRight size={17} /> : <CaretLeft size={17} />}</button></header>
    {!collapsed && <p className="sidebar-label">我的群组 <span>{groups.length}</span></p>}
    <nav className="group-list" aria-label="切换群组">
      {groups.map(group => <button key={group.id} title={group.name} className={`group-row ${selected?.id === group.id ? "is-active" : ""}`} aria-pressed={selected?.id === group.id} onClick={() => select(group.id)}><GroupIcon group={group} />{!collapsed && <span>{group.name}</span>}</button>)}
      {!collapsed && loading && <p role="status">正在读取群组…</p>}
      {!collapsed && !loading && !groups.length && !error && <p className="sidebar-empty">创建或加入一个群组，开始共同安排日程。</p>}
      {error && <div role="alert"><p>{error}</p><button onClick={() => void reload()}>重试</button></div>}
    </nav>
    <div className="sidebar-actions">
      <button title="创建群组" onClick={() => setPanel("create")}><Plus size={18} />{!collapsed && "创建群组"}</button>
      <button title="加入群组" onClick={() => setPanel("join")}><SignIn size={18} />{!collapsed && "加入群组"}</button>
      <button title="成员与邀请" disabled={!selected} onClick={() => setPanel("manage")}><Users size={18} />{!collapsed && "成员与邀请"}</button>
    </div>
    <button className="sidebar-account" title="个人设置" onClick={onSettings}><Avatar url={profile?.avatar} name={profile?.nickname || auth.user?.name || "我"} />{!collapsed && <span><strong>{profile?.nickname || auth.user?.name}</strong><small>{auth.user?.account}</small></span>}{!collapsed && <GearSix size={17} />}</button>
    {panel && <GroupPanel mode={panel} group={selected} close={() => setPanel(null)} />}
  </aside>;
}

function GroupPanel({ mode, group, close }: { mode: "create" | "join" | "manage"; group: Group | null; close: () => void }) {
  // 打开面板时固定对象，其他窗口切群不会改变正在确认的退出/解散目标。
  const [panelGroup] = useState(group);
  group = panelGroup;
  const { reload, select } = useGroups();
  const invite = parseInvite(window.location.href);
  const [name, setName] = useState("");
  const [groupID, setGroupID] = useState(String(invite?.groupID || ""));
  const [code, setCode] = useState(invite?.code || "");
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(mode === "manage");
  const [confirm, setConfirm] = useState<"quit" | "dismiss" | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const owner = Boolean(group && group.owner_id === auth.user?.id);
  useEffect(() => { dialog.current?.showModal(); return () => dialog.current?.close(); }, []);
  useEffect(() => {
    if (mode !== "manage" || !group) return;
    let active = true;
    Promise.all([groupMembers(group.id), groupInvite(group.id)]).then(([users, invitation]) => {
      if (active) { setMembers(users); setCode(invitation.code); }
    }).catch(reason => { if (active) setError(reason instanceof Error ? reason.message : "读取群组失败"); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [mode, group?.id]);
  async function run(operation: () => Promise<void>) {
    if (busy) return; setBusy(true); setError(""); setNotice("");
    try { await operation(); } catch (reason) { setError(reason instanceof Error ? reason.message : "操作失败"); }
    finally { setBusy(false); }
  }
  async function changed(id?: number) {
    await reload(); if (id) select(id); await publishDataChanged({ type: "groups" }); close();
  }
  const link = group ? invitationValue(group.id, code, window.location.origin) : "";
  return <dialog ref={dialog} className="group-dialog" onCancel={event => { if (busy) event.preventDefault(); else close(); }} aria-labelledby="group-dialog-title">
    <header><h2 id="group-dialog-title">{mode === "create" ? "创建群组" : mode === "join" ? "加入群组" : group?.name}</h2><button aria-label="关闭" disabled={busy} onClick={close}><X size={20} /></button></header>
    {mode === "create" && <form onSubmit={event => { event.preventDefault(); void run(async () => { const result = await createGroup(name.trim()); await changed(result.id); }); }}><p>群组中的日程与待办仅对当前群成员可见。</p><label>群组名称<input required autoFocus maxLength={80} value={name} disabled={busy} onChange={event => setName(event.target.value)} placeholder="例如：周末计划" /></label><button className="primary" disabled={busy || !name.trim()}>{busy ? "创建中…" : "创建群组"}</button></form>}
    {mode === "join" && <form onSubmit={event => { event.preventDefault(); void run(async () => { await joinGroup(Number(groupID), code.trim().toUpperCase()); await changed(Number(groupID)); }); }}><p>填写群组 ID 和邀请码，或粘贴邀请链接／卡片。</p><label>邀请链接或卡片<textarea disabled={busy} rows={3} placeholder="粘贴邀请信息自动填写" onChange={event => { const parsed = parseInvite(event.target.value); if (parsed) { setGroupID(String(parsed.groupID)); setCode(parsed.code); } }} /></label><label>群组 ID<input type="number" min={1} required value={groupID} disabled={busy} onChange={event => setGroupID(event.target.value)} /></label><label>邀请码<input required pattern="[A-Z0-9]{6}" maxLength={6} value={code} disabled={busy} onChange={event => setCode(event.target.value.toUpperCase())} placeholder="6 位数字或大写字母" /></label><button className="primary" disabled={busy}>{busy ? "加入中…" : "加入群组"}</button></form>}
    {mode === "manage" && group && <>
      <p className="group-boundary">群组 ID {group.id} · 所有成员可见群内日程与待办</p>
      {loading ? <p role="status">正在读取成员与邀请…</p> : <>
        {code && <section className="invite-card"><div><small>邀请好友加入</small><strong className="invite-code">{code}</strong><span>群组 ID：{group.id}</span><button disabled={busy} onClick={() => void run(async () => { await navigator.clipboard.writeText(link); setNotice("邀请信息已复制"); })}>复制邀请信息</button>{owner && <button disabled={busy} onClick={() => { if (window.confirm("刷新后旧邀请码和邀请链接会失效，继续吗？")) void run(async () => { const result = await refreshGroupInvite(group.id); setCode(result.code); }); }}>刷新邀请码</button>}</div><QRCodeSVG value={link} size={108} marginSize={2} title="群组邀请二维码" /></section>}
        <h3>成员 · {members.length}</h3><ul className="member-list">{members.map(member => <li key={member.id}><Avatar url={member.avatar} name={member.nickname || member.username} /><span><strong>{member.nickname || member.username}</strong><small>{member.username}#{member.suffix}</small></span><small>{member.id === group.owner_id ? "群主" : member.id === auth.user?.id ? "我" : "成员"}</small></li>)}</ul>
        {owner && <label>转让群主后退出<select value={target} onChange={event => { setTarget(event.target.value); setConfirm(null); }} disabled={busy}><option value="">选择新的群主</option>{members.filter(member => member.id !== auth.user?.id).map(member => <option key={member.id} value={member.id}>{member.nickname || member.username}</option>)}</select></label>}
        <div className="group-danger-actions"><button disabled={busy || (owner && !target)} onClick={() => setConfirm("quit")}>{owner ? "转让并退出群组" : "退出群组"}</button>{owner && <button disabled={busy} onClick={() => setConfirm("dismiss")}>解散群组</button>}</div>
        {confirm && <section className="group-confirm" role="alert"><p>{confirm === "dismiss" ? "解散后会删除群组内全部 Todo、Event 和完成记录。确认解散？" : owner ? "确认转让群主并退出？退出后将无法访问群内内容。" : "确认退出？退出后将无法访问群内内容。"}</p><button disabled={busy} onClick={() => void run(async () => { if (confirm === "dismiss") await dismissGroup(group.id); else await quitGroup(group.id, owner ? Number(target) : undefined); await changed(); })}>{busy ? "处理中…" : "确认"}</button><button disabled={busy} onClick={() => setConfirm(null)}>取消</button></section>}
      </>}
    </>}
    {error && <p className="form-error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  </dialog>;
}
