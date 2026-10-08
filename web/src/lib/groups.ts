const COLORS = ["#5276B8", "#A76F45", "#4D8A70", "#A45C80", "#7962AD", "#C07B35", "#508A9C"];
export function groupColor(id: number) { return COLORS[(Math.imul(id, 2654435761) >>> 0) % COLORS.length]; }
export function groupInitial(name: string) { return Array.from(name.trim())[0]?.toLocaleUpperCase() || "群"; }

// 群号公开且固定；链接和二维码只携带群号，不携带入群凭证。
export function parseInvite(value: string): { code: string } | null {
  let code = value.trim().toUpperCase();
  try {
    const url = new URL(value);
    if (!["https:", "http:"].includes(url.protocol)) return null;
    code = (url.searchParams.get("code") || "").toUpperCase();
  } catch {
    code = (value.match(/^群号：[ \t]*([A-Za-z0-9]{6})[ \t]*$/m)?.[1] || code).toUpperCase();
  }
  return /^[0-9A-Z]{6}$/.test(code) ? { code } : null;
}
export function invitationValue(code: string, origin: string) {
  const url = new URL(origin);
  if (["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)) return `Note 群组\n群号：${code}`;
  return `${url.origin}/?${new URLSearchParams({ code })}`;
}
