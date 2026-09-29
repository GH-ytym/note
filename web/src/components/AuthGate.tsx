import { useEffect, useState, useSyncExternalStore } from "react";
import type { FormEvent, ReactNode } from "react";
import { auth, APIError } from "../auth";
import WindowFrame from "../windows/WindowFrame";
import "../styles/auth.css";

export default function AuthGate({ children }: { children: ReactNode }) {
  const user = useSyncExternalStore(auth.subscribe, auth.snapshot);
  const [ready, setReady] = useState(auth.initialized);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [account, setAccount] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const restore = () => {
    setError(""); setBusy(true);
    void auth.restore().then(() => setReady(true))
      .catch(() => setError("暂时无法恢复登录，请检查网络或稍后重试。"))
      .finally(() => setBusy(false));
  };
  useEffect(restore, []);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError(""); setBusy(true);
    try {
      if (registering) {
        const response = await fetch("/api/auth/register", {
          method: "POST", credentials: "include",
          headers: { "Content-Type": "application/json", "X-Note-Request": "1" },
          body: JSON.stringify({ name, email: account, password }),
        });
        if (!response.ok) {
          const data = await response.json().catch(() => ({}));
          throw new APIError(data.error || "注册失败", response.status);
        }
        // 注册和登录是两个操作；登录失败时允许直接登录，避免重复注册。
        setRegistering(false);
      }
      await auth.login(account, password);
      setPassword(""); setReady(true);
    } catch (error) {
      setError(error instanceof Error ? error.message : "登录失败，请重试");
    } finally { setBusy(false); }
  };
  if (user) return <div className="authenticated-app" key={user.id}>{children}</div>;
  return (
    <WindowFrame title="Note" subtitle="日程与待办" className="auth-window">
      <section className="auth-card">
        {!ready ? <>
          <h2>正在恢复登录</h2>
          <p role="status">{error || "正在检查你的登录状态…"}</p>
          {error && <button type="button" disabled={busy} onClick={restore}>重新尝试</button>}
        </> : <>
          <h2>{registering ? "创建账号" : "欢迎回来"}</h2>
          <p>登录后继续安排你的日程。</p>
          <form onSubmit={submit}>
            {registering && <label>用户名<input autoComplete="nickname" required maxLength={20} value={name} onChange={e => setName(e.target.value)} /></label>}
            <label>{registering ? "邮箱" : "邮箱或完整账号"}<input type={registering ? "email" : "text"} autoComplete="username" required maxLength={254} value={account} onChange={e => setAccount(e.target.value)} placeholder={registering ? "name@example.com" : "邮箱或 用户名#12345"} /></label>
            <label>密码<input type="password" autoComplete={registering ? "new-password" : "current-password"} required maxLength={20} value={password} onChange={e => setPassword(e.target.value)} /></label>
            {registering && <small>密码为 8–20 位，请使用字母、数字或下划线。</small>}
            {error && <p className="auth-error" role="alert">{error}</p>}
            <button className="auth-primary" disabled={busy} type="submit">{busy ? "请稍候…" : registering ? "注册并登录" : "登录"}</button>
          </form>
          <button className="auth-switch" disabled={busy} type="button" onClick={() => { setRegistering(!registering); setError(""); }}>{registering ? "已有账号，去登录" : "没有账号？创建账号"}</button>
        </>}
      </section>
    </WindowFrame>
  );
}

export function AccountSettings() {
  const user = useSyncExternalStore(auth.subscribe, auth.snapshot);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const logout = async () => {
    setBusy(true); setError("");
    try { await auth.logout(); }
    catch { setError("退出未完成，请检查网络后重试。"); }
    finally { setBusy(false); }
  };
  return <div className="account-settings">
    <p>当前账号：<strong>{user?.account}</strong></p>
    <button disabled={busy} onClick={logout}>{busy ? "正在退出…" : "退出登录"}</button>
    {error && <p role="alert" className="auth-error">{error}</p>}
  </div>;
}
