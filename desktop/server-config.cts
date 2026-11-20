// 远程桌面端加载服务器提供的页面，API、登录和 SSE 因而保持同源。
export function serverURL(value: unknown): string {
  if (typeof value !== "string" || !value.trim()) throw new Error("请配置 Note 服务器地址");
  const url = new URL(value.trim());
  if (url.protocol !== "https:" && !(url.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname))) {
    throw new Error("远程服务器必须使用 HTTPS");
  }
  if (url.username || url.password || url.search || url.hash || !/^\/(?:[a-zA-Z0-9_-]+\/)*$/.test(url.pathname)) {
    throw new Error("服务器地址不能包含账号或参数；子路径必须以 / 结尾");
  }
  return url.pathname === "/" ? url.origin : url.href;
}

export function apiURL(base: string, path: string): string {
  return new URL(`api/${path.replace(/^\/+/, "")}`, base.endsWith("/") ? base : base + "/").toString();
}
