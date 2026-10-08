import { useEffect, useRef, useState } from "react";
import { auth } from "../auth";
import { consumeNotifications } from "../lib/notification-stream";

export interface NotificationCard {
  id: number;
  type: string;
  actor_id: number;
  actor_name: string;
  actor_avatar: string;
  group_id: number;
  group_name: string;
  request_status?: string;
  read_at: string | null;
  created_at: string;
}
interface Inbox { items: NotificationCard[]; unread_count: number }

export default function useNotifications() {
  const [inbox, setInbox] = useState<Inbox>({ items: [], unread_count: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [connected, setConnected] = useState(false);
  const reload = useRef<() => void>(() => {});
  useEffect(() => {
    const controller = new AbortController();
    const { signal } = controller;
    let running = false;
    let dirty = false;
    let retryTimer: ReturnType<typeof setTimeout>;
    // 请求期间再收到事件，记下来，完成后继续查；不并行覆盖列表。
    async function refresh() {
      dirty = true;
      if (running || signal.aborted) return;
      running = true;
      setLoading(true);
      try {
        while (dirty && !signal.aborted) {
          dirty = false;
          try {
            const result = await auth.request<Inbox>("/notifications", { signal });
            if (!signal.aborted) { setInbox(result); setError(""); }
          } catch (reason) {
            if (!signal.aborted) setError(reason instanceof Error ? reason.message : "通知加载失败");
            break;
          }
        }
      } finally {
        running = false;
        if (!signal.aborted) setLoading(false);
      }
    }
    reload.current = () => { void refresh(); };
    let retryDelay = 1000;
    async function connect() {
      try {
        const response = await auth.response("/notifications/stream", {
          signal, headers: { Accept: "text/event-stream" },
        });
        if (!response.body || !response.headers.get("content-type")?.includes("text/event-stream")) {
          await response.body?.cancel();
          throw new Error("通知连接响应无效");
        }
        if (signal.aborted) { await response.body.cancel(); return; }
        setConnected(true);
        retryDelay = 1000;
        // 后端已登记订阅并发送响应头；此后查询列表，事件会触发后续刷新。
        void refresh();
        await consumeNotifications(response.body, () => { void refresh(); });
      } catch {
        // 列表仍可独立使用。断线后带最新令牌重连，401 由认证层刷新。
      } finally {
        if (!signal.aborted) {
          setConnected(false);
          retryTimer = setTimeout(() => { void connect(); }, retryDelay);
          retryDelay = Math.min(retryDelay * 2, 30000);
        }
      }
    }
    void refresh();
    void connect();
    const onFocus = () => { void refresh(); };
    window.addEventListener("focus", onFocus);
    // 临时兜底：Redis 漏推时，即使 SSE 未断，列表也会定期校正。
    const poll = setInterval(onFocus, 60000);
    return () => {
      controller.abort();
      clearTimeout(retryTimer);
      clearInterval(poll);
      window.removeEventListener("focus", onFocus);
      reload.current = () => {};
    };
  }, []);
  return { ...inbox, loading, error, connected, reload: () => reload.current() };
}
