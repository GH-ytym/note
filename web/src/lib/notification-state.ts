export interface NotificationCard {
  id: number; type: string; actor_id: number; actor_name: string; actor_avatar: string;
  group_id: number; group_name: string; join_request_id?: number; request_status?: string;
  read_at: string | null; created_at: string;
}
export interface NotificationPage { revision: string; items: NotificationCard[]; unread_count: number; next: number }
export interface NotificationChange {
  revision: string; upserts: NotificationCard[]; removed_ids: number[];
  read_ids?: number[]; read_at?: string; unread_count: number;
}
export function version(value: string) {
  if (typeof value !== "string" || !/^(0|[1-9][0-9]*)$/.test(value)) throw new Error("通知版本无效");
  return BigInt(value);
}
export function applyChange(page: NotificationPage, change: NotificationChange): NotificationPage {
  const rev = version(change.revision), current = version(page.revision);
  if (rev <= current) return page;
  if (rev !== current + 1n) throw new Error("通知版本不连续，正在重新同步");
  const cards = new Map(page.items.map(card => [card.id, card]));
  const floor = page.next;
  for (const card of change.upserts) {
    if (cards.has(card.id) || floor === 0 || card.id > floor) cards.set(card.id, card);
  }
  for (const id of change.removed_ids) cards.delete(id);
  for (const id of change.read_ids ?? []) {
    const card = cards.get(id);
    if (card && change.read_at) cards.set(id, { ...card, read_at: change.read_at });
  }
  return { ...page, revision: change.revision, unread_count: change.unread_count,
    items: [...cards.values()].sort((a,b) => b.id-a.id) };
}
export function mergePage(current: NotificationPage, page: NotificationPage): NotificationPage {
  if (version(current.revision) !== version(page.revision)) throw new Error("列表已更新，请重新同步后加载");
  const cards = new Map(current.items.map(card => [card.id, card]));
  for (const card of page.items) if (!cards.has(card.id)) cards.set(card.id, card);
  return { ...current, next: page.next, items: [...cards.values()].sort((a,b)=>b.id-a.id) };
}
