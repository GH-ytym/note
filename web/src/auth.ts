import { APIError, AuthClient } from "./lib/auth-client";
import type { AuthTransport, AuthUser } from "./lib/auth-client";

const channel = !window.noteDesktop && typeof BroadcastChannel !== "undefined"
  ? new BroadcastChannel("note-auth") : null;

const transport: AuthTransport = async (action, body, token) => {
  if (window.noteDesktop) {
    const result = await window.noteDesktop.auth({ action, body, token });
    if (result.error) throw new APIError(result.error, result.status);
    return result.user;
  }
  const run = async () => {
    if (action === "refresh" && auth.user?.access_token !== token) return auth.user;
    const path = action === "restore" ? "refresh" : action;
    const response = await fetch(`/api/auth/${path}`, {
      method: "POST", credentials: "include",
      headers: { "Content-Type": "application/json", "X-Note-Request": "1" },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      if (response.status === 401 && (action === "refresh" || action === "restore")) {
        channel?.postMessage(null);
      }
      throw new APIError(data.error || "登录服务暂时不可用", response.status);
    }
    const user: AuthUser | null = action === "logout" ? null : await response.json();
    channel?.postMessage(user);
    return user;
  };
  // 同源浏览器标签页共用锁，Cookie 轮换与退出也不会互相插入。
  return navigator.locks ? navigator.locks.request("note-auth", run) : run();
};

export const auth = new AuthClient(transport);
channel?.addEventListener("message", event => auth.accept(event.data));
window.noteDesktop?.onAuthChanged(user => auth.accept(user));
export { APIError };
