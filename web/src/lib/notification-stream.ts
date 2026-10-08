// 只识别完整 SSE 事件，用作重新查询通知列表的信号。
// 不把历史 Card 快照覆盖到数据库返回的当前状态上。
export async function consumeNotifications(body: ReadableStream<Uint8Array>, changed: () => void) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let eventName = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buffer += decoder.decode(value, { stream: true });
      let newline: number;
      while ((newline = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, newline).replace(/\r$/, "");
        buffer = buffer.slice(newline + 1);
        if (!line) {
          if (eventName === "notification.created" || eventName === "notification.updated") changed();
          eventName = "";
        } else if (line.startsWith("event:")) {
          eventName = line.slice(6).trim();
        }
      }
      if (buffer.length > 1024 * 1024) throw new Error("通知消息过大");
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}
