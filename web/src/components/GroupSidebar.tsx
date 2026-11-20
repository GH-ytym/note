import { useEffect, useRef, useState, lazy, Suspense } from "react";
import { Plus, Users, GearSix, CaretLeft, CaretRight, SignIn, X, DotsThree } from "@phosphor-icons/react";
import { createPortal } from "react-dom";
import { QRCodeSVG } from "qrcode.react";
import { auth } from "../auth";
import { useGroups } from "../groups";
import { useProfile } from "../profile";
import type { Group, GroupMember } from "../types";
import { createGroup, joinGroup, groupMembers, lookupGroup, quitGroup, dismissGroup } from "../api";
import { publishDataChanged } from "../data-events";
import { groupColor, groupInitial, parseInvite, invitationValue } from "../lib/groups";
import type { GroupPreview } from "../api";
import Avatar from "./Avatar";
const CodeSlots = lazy(() => import("./react-bits/CodeSlots"));
import NotificationMenu from "./NotificationMenu";

import "../styles/groups.css";

export function GroupIcon({ group, size }: { group: Pick<Group, "id" | "name">; size?: number }) {
  return <span className="group-icon" style={{ backgroundColor: groupColor(group.id), ...(size ? {width:size, height:size, flexBasis:size, fontSize:size * .45} : {}) }} aria-hidden="true">{groupInitial(group.name)}</span>;
}

export function openGroupInfo(id: number) { window.dispatchEvent(new CustomEvent("note-group-info", {detail:id})); }

export default function GroupSidebar({ onSettings, disabled = false }: { onSettings: () => void; disabled?: boolean }) {
  const { groups, selected, select, loading, error, reload } = useGroups();
  const { profile } = useProfile();
  const [info, setInfo] = useState<Group | null>(null);
 useEffect(() => {
 const open = (e: Event) => { const item = groups.find(g => g.id === (e as CustomEvent<number>).detail); if (item) setInfo(item); };
 window.addEventListener("note-group-info", open);
 return () => window.removeEventListener("note-group-info", open);
 }, [groups]);
 const [collapsed, setCollapsed] = useState(false);
  const [panel, setPanel] = useState<"create" | "join" | "manage" | null>(() => parseInvite(window.location.href) ? "join" : null);
  return <aside inert={disabled} className={`group-sidebar ${collapsed ? "is-collapsed" : ""}`} aria-label="群组侧边栏">
    <header className="group-sidebar-header"><strong><span className="note-mark">N</span>{!collapsed && "Note"}</strong><button title={collapsed ? "展开侧边栏" : "收起侧边栏"} aria-label={collapsed ? "展开侧边栏" : "收起侧边栏"} onClick={() => setCollapsed(!collapsed)}>{collapsed ? <CaretRight size={17} /> : <CaretLeft size={17} />}</button></header>

    <nav className="group-list" aria-label="切换群组">
      {groups.map(group => <div className="group-list-entry" key={group.id}><button title={group.name} className={`group-row ${selected?.id === group.id ? "is-active" : ""}`} aria-pressed={selected?.id === group.id} onClick={() => select(group.id)}><GroupIcon group={group} size={24}/>{!collapsed && <span>{group.name}</span>}</button><button className="group-info-trigger" title={`查看${group.name}资料`} aria-label={`查看${group.name}资料`} onClick={() => setInfo(group)}><DotsThree size={20}/></button></div>)}
      {!collapsed && loading && <p role="status">正在读取群组…</p>}
      {!collapsed && !loading && !groups.length && !error && <p className="sidebar-empty">创建或加入一个群组，开始共同安排日程。</p>}
      {error && <div role="alert"><p>{error}</p><button onClick={() => void reload()}>重试</button></div>}
    </nav>
    <div className="sidebar-actions">
      <button title="创建群组" onClick={() => setPanel("create")}><Plus size={18} />{!collapsed && "创建群组"}</button>
      <button title="加入群组" onClick={() => setPanel("join")}><SignIn size={18} />{!collapsed && "加入群组"}</button>
      <button title="群组资料" disabled={!selected} onClick={() => setInfo(selected)}><Users size={18} />{!collapsed && "群组资料"}</button>
    </div>
    <div className="sidebar-account">
      <div className="sidebar-profile">
        <Avatar url={profile?.avatar} name={profile?.nickname || auth.user?.name || "我"} />
        {!collapsed && <span><strong>{profile?.nickname || auth.user?.name}</strong><small>{auth.user?.account}</small></span>}
      </div>
      <div className="sidebar-account-actions">
        <NotificationMenu collapsed />
        <button title="个人设置" aria-label="个人设置" onClick={onSettings}><GearSix size={17} /></button>
      </div>
    </div>

    {info && createPortal(<GroupPanel key={info.id} mode="manage" group={info} close={() => setInfo(null)} />, document.body)}
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
  const [policy, setPolicy] = useState<"public" | "approval" | "restricted">("approval");
  const [finding, setFinding] = useState(false);
  const [preview, setPreview] = useState<GroupPreview | null>(null);
  const [code, setCode] = useState(mode === "join" ? invite?.code || "" : group?.code || "");
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(mode === "manage");
  const [confirm, setConfirm] = useState<"quit" | "dismiss" | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const owner = Boolean(group && group.owner_id === auth.user?.id);
  const personal = group?.policy === "personal";
  useEffect(() => { const el = dialog.current; if (mode === "manage") el?.show(); else el?.showModal();
 return () => el?.close(); }, [mode]);
  useEffect(() => {
    if (mode !== "manage" || !group) return;
    let active = true;
    groupMembers(group.id).then(users => {
      if (active) { setMembers(users); }
    }).catch(reason => { if (active) setError(reason instanceof Error ? reason.message : "读取群组失败"); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [mode, group?.id]);
  useEffect(() => {
    if (mode !== "join") return;
    let active = true;
    setPreview(null); setError(""); setNotice("");
    const parsed = parseInvite(code);
    if (!parsed) { setFinding(false); return; }
    setFinding(true);
    lookupGroup(parsed.code).then(item => { if (active) setPreview(item); })
      .catch(reason => { if (active) setError(reason instanceof Error ? reason.message : "查找失败"); })
      .finally(() => { if (active) setFinding(false); });
    return () => { active = false; };
  }, [code, mode]);
  async function run(operation: () => Promise<void>) {
    if (busy) return; setBusy(true); setError(""); setNotice("");
    try { await operation(); } catch (reason) { setError(reason instanceof Error ? reason.message : "操作失败"); }
    finally { setBusy(false); }
  }
  async function changed(id?: number) {
    await reload(); if (id) select(id); await publishDataChanged({ type: "groups" }); close();
  }
  const link = group ? invitationValue(group.code, window.location.origin) : "";
  return <dialog ref={dialog} className={`group-dialog ${mode === "manage" ? "group-info-drawer" : ""}`} onCancel={event => { if (busy) event.preventDefault(); else close(); }} aria-labelledby="group-dialog-title" onKeyDown={event => { if (mode === "manage" && event.key === "Escape" && !busy) { event.preventDefault(); close(); } }}>
    <header><h2 id="group-dialog-title">{mode === "create" ? "创建群组" : mode === "join" ? "加入群组" : group?.name}</h2><button aria-label="关闭" disabled={busy} onClick={close}><X size={20} /></button></header>
    {mode === "create" && <form onSubmit={event => { event.preventDefault(); void run(async () => { const result = await createGroup(name.trim(), policy); await changed(result.id); }); }}><p>群组中的日程与待办仅对当前群成员可见。</p><label>群组名称<input required autoFocus maxLength={80} value={name} disabled={busy} onChange={event => setName(event.target.value)} placeholder="例如：周末计划" /></label><fieldset className="group-policy"><legend>加入方式</legend>{([
        ["public", "自由加入", "通过群号直接加入"],
        ["approval", "需要审核", "提交申请，由群主审核"],
        ["restricted", "仅限邀请", "仅群主可邀请新成员"],
      ] as const).map(([value, title, hint]) => <label className={policy === value ? "is-selected" : ""} key={value}><input type="radio" name="group-policy" value={value} checked={policy === value} disabled={busy} onChange={() => setPolicy(value)} /><span><strong>{title}</strong><small>{hint}</small></span></label>)}</fieldset><button className="primary" disabled={busy || !name.trim()}>{busy ? "创建中…" : "创建群组"}</button></form>}
    {mode === "join" && <form onSubmit={event => { event.preventDefault(); if (!preview || finding) return; void run(async () => {
      const result = await joinGroup(preview.code);
      if (result.status === "pending") setNotice("申请已发送，等待群主审核。");
      else { await reload(); setNotice("已加入群组，可以从左侧列表进入。"); }
    }); }}>
      <p>输入六位群号，支持数字和字母。</p>
      <Suspense fallback={<p role="status">正在加载输入框…</p>}><CodeSlots value={code} onChange={value => { setCode(value); setPreview(null); setNotice(""); }} length={6} autoFocus disabled={busy}
        ariaLabel="六位群号" accentColor="var(--yellow)" inkColor="var(--text)" slotColor="var(--cell-hover)" digitColor="var(--text)" slotSize={42} gap={8} /></Suspense>
      {finding && <p role="status">正在查找群组…</p>}
      <div className={`group-preview-expand ${preview ? "is-open" : ""}`}><div>
        {preview && <section className="invite-card"><div><strong>{preview.name}</strong><span>{preview.member_count} 位成员</span><span className="preview-owner"><Avatar url={preview.owner_avatar} name={preview.owner_name} />群主 · {preview.owner_name}</span><small>{preview.policy === "public" ? "自由加入" : preview.policy === "approval" ? "需要群主审核" : "仅限群主邀请"}</small></div></section>}
      </div></div>
      {preview && <button className="primary" disabled={busy || finding || !!notice || preview.policy === "restricted"}>{busy ? "处理中…" : preview?.policy === "approval" ? "申请加入" : "加入群组"}</button>}
    </form>}
    {mode === "manage" && group && <>
      <section className="group-info-hero"><GroupIcon group={group} size={64}/><div><h3>{group.name}</h3><p>{personal ? "个人空间" : owner ? "我是群主" : "我是成员"} · {({public:"自由加入",approval:"需要审核",restricted:"仅限邀请",personal:"仅自己可见"})[group.policy]}</p></div></section>
      <p className="group-boundary">{personal ? "你的私人日程与待办" : "所有成员可见群内日程与待办"}</p>
      {loading ? <p role="status">正在读取成员与邀请…</p> : <>
        {group.policy !== "personal" && code && <section className="invite-card"><div><small>分享群号</small><strong className="invite-code">{code}</strong><span>群号固定不变</span><button disabled={busy} onClick={() => void run(async () => { await navigator.clipboard.writeText(link); setNotice("群号已复制"); })}>复制群号／链接</button></div><QRCodeSVG value={link} size={108} marginSize={2} title="群号二维码" /></section>}
        <div className="group-owner"><small>群主</small>{members.filter(m => m.id === group.owner_id).map(m => <span key={m.id}><Avatar url={m.avatar} name={m.nickname || m.username}/>{m.nickname || m.username}</span>)}</div><h3>成员 · {members.length}</h3><ul className="member-list">{members.map(member => <li key={member.id}><Avatar url={member.avatar} name={member.nickname || member.username} /><span><strong>{member.nickname || member.username}</strong><small>{member.username}#{member.suffix}</small></span><small>{member.id === group.owner_id ? "群主" : member.id === auth.user?.id ? "我" : "成员"}</small></li>)}</ul>
        {owner && !personal && <label>转让群主后退出<select value={target} onChange={event => { setTarget(event.target.value); setConfirm(null); }} disabled={busy}><option value="">选择新的群主</option>{members.filter(member => member.id !== auth.user?.id).map(member => <option key={member.id} value={member.id}>{member.nickname || member.username}</option>)}</select></label>}
        {!personal && <div className="group-danger-actions"><button disabled={busy || (owner && !target)} onClick={() => setConfirm("quit")}>{owner ? "转让并退出群组" : "退出群组"}</button>{owner && <button disabled={busy} onClick={() => setConfirm("dismiss")}>解散群组</button>}</div>}
        {confirm && <section className="group-confirm" role="alert"><p>{confirm === "dismiss" ? "解散后会删除群组内全部 Todo、Event 和完成记录。确认解散？" : owner ? "确认转让群主并退出？退出后将无法访问群内内容。" : "确认退出？退出后将无法访问群内内容。"}</p><button disabled={busy} onClick={() => void run(async () => { if (confirm === "dismiss") await dismissGroup(group.id); else await quitGroup(group.id, owner ? Number(target) : undefined); await changed(); })}>{busy ? "处理中…" : "确认"}</button><button disabled={busy} onClick={() => setConfirm(null)}>取消</button></section>}
      </>}
    </>}
    {error && <p className="form-error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  </dialog>;
}
