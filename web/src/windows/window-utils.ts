import type { DataChange } from "../desktop";
import { validDate } from "../lib/calendar";
import { publishDataChanged } from "../data-events";

export function windowParams() {
  return new URLSearchParams(window.location.search);
}

export function closeCurrentWindow() {
  if (window.noteDesktop?.closeCurrent) {
    void window.noteDesktop.closeCurrent();
    return;
  }
  window.close();
}

export async function notifyDataChanged(payload: DataChange) {
  await publishDataChanged(payload);
}

export function initialDateFromWindow() {
  return validDate(windowParams().get("date"));
}
