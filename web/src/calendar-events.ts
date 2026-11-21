import type { CalendarChange } from "./lib/calendar-change";

// 每个窗口已有自己的 SSE；只通知本窗口，避免跨窗口再次广播形成重复刷新。
// null 表示 SSE 已重新建立，需要校正当前范围。
export function publishCalendarSync(change: CalendarChange | null) {
  window.dispatchEvent(new CustomEvent("note-calendar-sync", { detail: change }));
}

export function onCalendarSync(receive: (change: CalendarChange | null) => void) {
  const listener = (e: Event) => receive((e as CustomEvent<CalendarChange | null>).detail);
  window.addEventListener("note-calendar-sync", listener);
  return () => window.removeEventListener("note-calendar-sync", listener);
}
