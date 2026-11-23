// 详情显示后标记当前卡片；重复渲染共用请求，失败允许重新尝试。
export function createReadMarker(mark: (ids: number[]) => Promise<void>) {
  const done = new Set<number>();
  const pending = new Map<number, Promise<void>>();
  return (card: { id: number; read_at?: string | null }): Promise<void> => {
    if (card.read_at || done.has(card.id)) return Promise.resolve();
    const old = pending.get(card.id);
    if (old) return old;
    const request = Promise.resolve().then(() => mark([card.id])).then(() => {
      done.add(card.id);
    }).finally(() => pending.delete(card.id));
    pending.set(card.id, request);
    return request;
  };
}
