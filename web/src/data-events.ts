import type { DataChange } from "./desktop";

const channel = !window.noteDesktop && typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("note-data") : null;
channel?.addEventListener("message", event => window.dispatchEvent(new CustomEvent("note-data", { detail: event.data })));

export function onDataChanged(listener: (value: DataChange) => void) {
  const receive = (event: Event) => listener((event as CustomEvent<DataChange>).detail);
  window.addEventListener("note-data", receive);
  const remove = window.noteDesktop?.onDataChanged?.(listener);
  return () => { window.removeEventListener("note-data", receive); remove?.(); };
}
export async function publishDataChanged(payload: DataChange) {
  window.dispatchEvent(new CustomEvent("note-data", { detail: payload }));
  channel?.postMessage(payload);
  await window.noteDesktop?.notifyDataChanged?.(payload);
}
