export interface AuthUser {
  id: number;
  name: string;
  suffix: number;
  account: string;
  access_token: string;
  token_type: string;
}

export class APIError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

type Action = "restore" | "refresh" | "login" | "logout";
export type AuthTransport = (action: Action, body?: Record<string, string>, token?: string) => Promise<AuthUser | null>;

// 所有令牌只存于内存。刷新、登录、退出共用队列，退出不会被旧刷新覆盖。
export class AuthClient {
  user: AuthUser | null = null;
  initialized = false;
  private revision = 0;
  private queue: Promise<unknown> = Promise.resolve();
  private restoring: Promise<void> | null = null;
  private listeners = new Set<() => void>();
  private transport: AuthTransport;
  constructor(transport: AuthTransport) { this.transport = transport; }
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; };
  snapshot = () => this.user;
  accept(user: AuthUser | null) {
    this.user = user;
    this.initialized = true;
    this.revision++;
    this.listeners.forEach(listener => listener());
  }
  private serial<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.queue.then(operation);
    this.queue = result.catch(() => {});
    return result;
  }
  restore(): Promise<void> {
    if (this.initialized) return Promise.resolve();
    if (this.restoring) return this.restoring;
    this.restoring = this.serial(async () => {
      if (this.initialized) return;
      try { this.accept(await this.transport("restore")); }
      catch (error) {
        if (error instanceof APIError && error.status === 401) this.accept(null);
        else throw error;
      }
    }).finally(() => { this.restoring = null; });
    return this.restoring;
  }
  login(account: string, password: string) {
    return this.serial(async () => { this.accept(await this.transport("login", { account, password })); });
  }
  logout() {
    return this.serial(async () => {
      await this.transport("logout");
      this.accept(null);
    });
  }
  refresh(rejectedToken: string) {
    return this.serial(async () => {
      if (!this.user) throw new APIError("请重新登录", 401);
      if (this.user.access_token !== rejectedToken) return;
      try { this.accept(await this.transport("refresh", undefined, rejectedToken)); }
      catch (error) {
        if (error instanceof APIError && error.status === 401) this.accept(null);
        throw error;
      }
    });
  }
  async request<T>(path: string, options: RequestInit = {}, fetcher: typeof fetch = fetch): Promise<T> {
    const response = await this.response(path, options, fetcher);
    if (response.status === 204) return null as T;
    return response.json();
  }
  // SSE 需要读取响应流；与普通请求共用认证和 401 刷新逻辑。
  async response(path: string, options: RequestInit = {}, fetcher: typeof fetch = fetch): Promise<Response> {
    const user = this.user;
    if (!user) throw new APIError("请先登录", 401);
    const revision = this.revision;
    const token = user.access_token;
    const send = (accessToken: string) => {
      const headers = new Headers(options.headers);
      if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
      headers.set("Authorization", `Bearer ${accessToken}`);
      return fetcher(`/api${path}`, { ...options, headers, credentials: "include" });
    };
    let response = await send(token);
    if (!this.user || this.user.id !== user.id) throw new APIError("登录状态已变更", 401);
    if (response.status === 401) {
      // 不允许退出/重新登录后的旧业务请求触发刷新或重试。
      if (revision !== this.revision && this.user.access_token === token) throw new APIError("登录状态已变更", 401);
      await this.refresh(token);
      if (!this.user || this.user.id !== user.id) throw new APIError("登录状态已变更", 401);
      response = await send(this.user.access_token);
    }
    if (!this.user || this.user.id !== user.id) throw new APIError("登录状态已变更", 401);
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new APIError(body.error || "请求失败，请稍后重试", response.status);
    }
    return response;
  }
}
