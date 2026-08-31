import { validDate } from "../lib/calendar";

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

export async function notifyDataChanged(payload) {
  await window.noteDesktop?.notifyDataChanged?.(payload);
}

export function initialDateFromWindow() {
  return validDate(windowParams().get("date"));
}
