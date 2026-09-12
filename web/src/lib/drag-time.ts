import type { CalendarItem } from "../types";

export type TimeEdge = "start" | "end";
export function dragTimeLabel(item: CalendarItem, edge: TimeEdge, delta: number) {
  const end = item.kind === "event" && edge === "end";
  const value = end ? item.endsAt || `${item.endDate}T${item.endTime}:00+08:00`
    : item.startsAt || `${item.date}T${item.time}:00+08:00`;
  // Calendar uses Shanghai (UTC+08:00); use UTC formatting after applying that offset.
  return new Date(Date.parse(value) + (delta + 480) * 60000).toISOString().slice(0, 16).replace("T", " ");
}
