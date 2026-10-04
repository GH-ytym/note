const COLORS = ["#5276B8", "#A76F45", "#4D8A70", "#A45C80", "#7962AD", "#C07B35", "#508A9C"];
export function groupColor(id: number) { return COLORS[(Math.imul(id, 2654435761) >>> 0) % COLORS.length]; }
export function groupInitial(name: string) { return Array.from(name.trim())[0]?.toLocaleUpperCase() || "群"; }

export function parseInvite(value: string): { groupID: number; code: string } | null {
  let groupID = 0, code = "";
  try {
    const url = new URL(value);
    if (!["https:", "http:"].includes(url.protocol)) return null;
    groupID = Number(url.searchParams.get("invite_group"));
    code = (url.searchParams.get("code") || "").toUpperCase();
  } catch {
    groupID = Number(value.match(/^群组 ID：[ \t]*(\d+)[ \t]*$/m)?.[1]);
    code = (value.match(/^邀请码：[ \t]*([A-Za-z0-9]{6})[ \t]*$/m)?.[1] || "").toUpperCase();
  }
  return Number.isSafeInteger(groupID) && groupID > 0 && /^[0-9A-Z]{6}$/.test(code) ? { groupID, code } : null;
}

export function invitationValue(groupID: number, code: string, origin: string) {
  const url = new URL(origin);
  // 本机地址无法分享给其他电脑；开发时分享群组 ID + 邀请码卡片。
  if (["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)) return `Note 群组邀请\n群组 ID：${groupID}\n邀请码：${code}`;
  return `${url.origin}/?${new URLSearchParams({ invite_group: String(groupID), code })}`;
}
