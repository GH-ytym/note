import { useEffect, useState } from "react";
import type { GroupMember, MemberPermission } from "../types";
import { groupMembers, patchMemberRoles } from "../api";
import { notifyDataChanged } from "../windows/window-utils";
import Avatar from "./Avatar";

export default function SchedulePermissions({ kind, id, groupID, creatorID, roles, onChanged }: {
  kind: "todo" | "event"; id: number; groupID: number; creatorID: number; roles: MemberPermission[]; onChanged: () => Promise<void>;
}) {
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    groupMembers(groupID).then(result => { if (active) setMembers(result); }).catch(reason => { if (active) setError(reason instanceof Error ? reason.message : "读取成员失败"); });
    return () => { active = false; };
  }, [groupID]);
  const roleMap = new Map((roles || []).map(role => [role.user_id, role.role]));
  async function update(role: 1 | 2) {
    if (!selected.length || busy) return;
    if (selected.length > 100) { setError("每次最多调整 100 位成员"); return; }
    setBusy(true); setError("");
    try { await patchMemberRoles(kind, id, selected, role); await onChanged(); setSelected([]); await notifyDataChanged({ type: "permissions" }); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "修改权限失败"); }
    finally { setBusy(false); }
  }
  return <section className="schedule-permissions" aria-label="管理成员权限">
    <h3>成员权限</h3><small>所有群成员可见；只有创建者可以调整编辑权限。</small>
    <ul className="member-list">{members.map(member => <li key={member.id}>
      <input type="checkbox" aria-label={`选择${member.nickname || member.username}`} disabled={busy || member.id === creatorID} checked={selected.includes(member.id)} onChange={event => setSelected(value => event.target.checked ? [...value, member.id] : value.filter(id => id !== member.id))} />
      <Avatar url={member.avatar} name={member.nickname || member.username} size={26} /><span><strong>{member.nickname || member.username}</strong></span><small>{member.id === creatorID ? "创建者" : roleMap.get(member.id) === "editor" ? "可编辑" : "只读"}</small>
    </li>)}</ul>
    <div className="role-controls"><button type="button" disabled={busy || !selected.length} onClick={() => void update(1)}>设为 editor</button><button type="button" disabled={busy || !selected.length} onClick={() => void update(2)}>设为 viewer</button><small>已选 {selected.length} 人</small></div>
    {error && <p role="alert" className="form-error">{error}</p>}
  </section>;
}
