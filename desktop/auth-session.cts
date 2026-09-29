import type { AuthCommand, AuthResult, AuthUser } from "./contracts.cjs";

// 所有窗口和提醒请求共享一个内存会话。Cookie 由 Electron Session 持久保存。
export class AuthSession {
  user: AuthUser | null = null;
  private initialized = false;
  private queue: Promise<unknown> = Promise.resolve();
  constructor(
    private send: (path: string, body?: Record<string, string>) => Promise<AuthResult>,
    private changed: (user: AuthUser | null) => void,
  ) {}
  run(command: AuthCommand): Promise<AuthResult> {
    const result = this.queue.then(() => this.execute(command));
    this.queue = result.catch(() => {});
    return result;
  }
  private async execute(command: AuthCommand): Promise<AuthResult> {
    const { action, token, body } = command;
    if (action === "restore" && this.initialized) return { user: this.user, status: this.user ? 200 : 401 };
    if (action === "refresh" && this.initialized) {
      if (!this.user) return { user: null, status: 401, error: "请重新登录" };
      if (token !== this.user.access_token) return { user: this.user, status: 200 };
    }
    let result: AuthResult;
    try { result = await this.send(action === "restore" ? "refresh" : action, body); }
    catch { return { user: this.user, status: 503, error: "无法连接登录服务，请重试" }; }
    if (result.status < 400 || (result.status === 401 && (action === "restore" || action === "refresh"))) {
      this.user = result.user;
      this.initialized = true;
      this.changed(this.user);
    }
    return result;
  }
}
