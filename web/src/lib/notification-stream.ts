export interface NotificationEvent { name: string; data: unknown }
export async function consumeNotifications(body: ReadableStream<Uint8Array>, receive: (event: NotificationEvent) => void) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "", name = "", lines: string[] = [];
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
          if (name === "notification.snapshot" || name === "notification.change" || name === "cldr") {
            receive({ name, data: JSON.parse(lines.join("\n")) });
          }
          name = ""; lines = [];
        } else if (line.startsWith("event:")) name = line.slice(6).trim();
        else if (line.startsWith("data:")) lines.push(line.slice(5).replace(/^ /, ""));
      }
    }
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}
