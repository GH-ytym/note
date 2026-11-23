import { useContext, useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Users } from "@phosphor-icons/react";
import { auth } from "../auth";
import { useProfile } from "../profile";
import { useGroups } from "../groups";
import type { NotificationCard } from "../hooks/useNotifications";
import { notificationAction } from "../lib/notification-review";
import Avatar from "./Avatar";
import NotificationReview from "./NotificationReview";
import { InboxContext, openPage } from "./Workspace";
import { createReadMarker } from "../lib/notification-read";
import { openGroupInfo, GroupIcon } from "./GroupSidebar";

type Detail = { card: NotificationCard; group: {id:number; name:string; policy:string; member_count:number} };
export default function NotificationDetail({ card: initial, active }: {card: NotificationCard; active:boolean}) {
 const { profile } = useProfile();
 const { groups } = useGroups();
 const [data, setData] = useState<Detail | null>(null);
 const [error, setError] = useState("");
 const [loading, setLoading] = useState(true);
 const [retry, setRetry] = useState(0);
 const inbox = useContext(InboxContext)!;
 const mark = useRef(inbox.markRead);
 mark.current = inbox.markRead;
 const [markViewed] = useState(() => createReadMarker(ids => mark.current(ids)));
 const [readError, setReadError] = useState("");
 const [readRetry, setReadRetry] = useState(0);
 useEffect(() => {
  if (!active || !data || data.card.id !== initial.id) return;
  let alive = true;
  setReadError("");
  void markViewed(data.card).catch(e => {
   if (alive) setReadError(e instanceof Error ? e.message : "标记已读失败");
  });
  return () => { alive = false; };
 }, [active, data, initial.id, markViewed, readRetry]);
 useEffect(() => {
  if (!active) return;
  let alive = true;
  const load = () => {
   setLoading(true);
   auth.request<Detail>(`/notifications/${initial.id}`).then(value => { if (alive) { setData(value); setError(""); } })
    .catch(e => { if (alive) setError(e instanceof Error ? e.message : "读取详情失败"); })
    .finally(() => { if (alive) setLoading(false); });
  };
  load(); window.addEventListener("focus", load);
  return () => { alive = false; window.removeEventListener("focus", load); };
 }, [initial.id, initial.request_status, active, retry]);
 const card = data?.card ?? initial;
 const member = groups.some(g => g.id === card.group_id);
 return <article className="notification-detail notification-story">
  <button className="story-back" onClick={() => openPage({id:"notifications", title:"通知"})}><ArrowLeft size={16}/>全部通知</button>
  <div className="story-people"><div><Avatar url={card.actor_avatar} name={card.actor_name} size={64}/><span>{card.actor_name}</span></div><div className="story-link" aria-hidden="true"><i/><ArrowRight size={18}/></div><div><Avatar url={profile?.avatar} name={profile?.nickname || "我"} size={64}/><span>{profile?.nickname || "我"}</span></div></div>
  <h1>{card.type === "join_requested" ? "新的入群申请" : card.type === "join_reviewed" ? "你的申请有了结果" : "新的群成员"}</h1>
  <p className="story-message"><strong>{card.actor_name}</strong> {notificationAction(card)}「{data?.group.name ?? card.group_name}」</p>
  <time>{new Date(card.created_at).toLocaleString("zh-CN")}</time>
  {loading && <p role="status">正在更新详情…</p>}
  {error && <p role="alert">{error} <button onClick={() => setRetry(n => n+1)}>重试</button></p>}
  {readError && <p role="alert">标记已读失败：{readError} <button onClick={() => setReadRetry(n => n+1)}>重试标记已读</button></p>}
  <NotificationReview card={card} fresh disabled={loading || !!error}/>
  {data && <section className="story-group"><GroupIcon group={data.group} size={44}/><div><strong>{data.group.name}</strong><small><Users size={14}/> {data.group.member_count} 位成员 · {({public:"自由加入",approval:"需要审核",restricted:"仅限邀请",personal:"个人空间"} as Record<string,string>)[data.group.policy]}</small></div>{member && <button onClick={() => openGroupInfo(card.group_id)}>群资料 <ArrowRight size={14}/></button>}</section>}
  {data && !member && <p className="story-hint">加入群组后可以查看成员、日程与待办。</p>}
 </article>;
}
