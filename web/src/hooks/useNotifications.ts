import { useEffect, useRef, useState } from "react";
import { auth } from "../auth";
import { consumeNotifications } from "../lib/notification-stream";
import { applyChange, mergePage, version } from "../lib/notification-state";
import type { NotificationPage, NotificationChange } from "../lib/notification-state";
export type { NotificationCard } from "../lib/notification-state";

const empty: NotificationPage = { revision: "0", items: [], unread_count: 0, next: 0 };
export default function useNotifications() {
  const [inbox, setInbox] = useState(empty);
  const current = useRef(empty);
  const generation = useRef(0);
  const ready = useRef(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const moreBusy = useRef(false);
  const [error, setError] = useState("");
  const [connected, setConnected] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const lifetime = useRef<AbortSignal | null>(null);
  const reload = () => { generation.current++; ready.current = false; setConnected(false); setAttempt(n=>n+1); };
  function update(page: NotificationPage) { current.current=page; setInbox(page); }
  useEffect(() => {
    const controller = new AbortController();
    const {signal}=controller;
    lifetime.current=signal;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let delay=1000;
    async function connect() {
      generation.current++; ready.current=false; setConnected(false); setLoading(true);
      try {
        const response=await auth.response("/notifications/stream", {signal,headers:{Accept:"text/event-stream"}});
        if (!response.body || !response.headers.get("content-type")?.includes("text/event-stream")) throw new Error("通知连接响应无效");
        await consumeNotifications(response.body,event=>{
          if(signal.aborted)return;
          if(event.name==="notification.snapshot"){
            const page=event.data as NotificationPage;
            version(page.revision);
            if(!Array.isArray(page.items)||!Number.isSafeInteger(page.next)||page.next<0||!Number.isSafeInteger(page.unread_count)||page.unread_count<0)throw new Error("通知快照无效");
            generation.current++; update(page); ready.current=true;
            setConnected(true);setLoading(false);setError("");delay=1000;
          }else{
            if(!ready.current)throw new Error("缺少通知快照");
            update(applyChange(current.current,event.data as NotificationChange));
          }
        });
      }catch(reason){
        if(!signal.aborted)setError(reason instanceof Error?reason.message:"通知连接失败");
      }finally{
        if(!signal.aborted){
          ready.current=false;generation.current++;setConnected(false);setLoading(false);
          timer=setTimeout(()=>void connect(),delay);delay=Math.min(delay*2,30000);
        }
      }
    }
    void connect();
    return()=>{controller.abort();clearTimeout(timer);generation.current++;ready.current=false;};
  },[attempt]);
  async function loadMore(){
    const page=current.current, gen=generation.current, signal=lifetime.current;
    if(moreBusy.current||!ready.current||!page.next||!signal)return;
    moreBusy.current=true;setLoadingMore(true);
    try{
      const next=await auth.request<NotificationPage>("/notifications?before="+page.next+"&limit=20",{signal});
      if(signal.aborted||generation.current!==gen)return;
      try{update(mergePage(current.current,next));setError("");}
      catch{reload();setError("通知已更新，已重新同步第一页，请再次加载");}
    }catch(reason){if(!signal.aborted)setError(reason instanceof Error?reason.message:"加载失败");}
    finally{moreBusy.current=false;setLoadingMore(false);}
  }
  async function markRead(ids:number[]){
    await auth.request<void>("/notifications/read",{method:"POST",body:JSON.stringify({ids}),signal:lifetime.current??undefined});
    reload();
  }
  return {...inbox,loading,loadingMore,error,connected,reload,loadMore,markRead};
}
