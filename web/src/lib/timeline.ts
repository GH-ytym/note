import type { CalendarItem } from "../types";

// All calendar dates use the application's Asia/Shanghai time zone.
const DAY = 86400000;
export function dayStart(key: string) { return Date.parse(`${key}T00:00:00+08:00`); }
export function itemBounds(item: CalendarItem) {
  const start = Date.parse(item.startsAt || `${item.date}T${item.time}:00+08:00`);
  const end = item.kind === "event" ? Date.parse(item.endsAt || `${item.endDate}T${item.endTime}:00+08:00`) : start;
  return { start, end };
}
export function occursOnDay(item: CalendarItem, key: string) {
  if (item.kind !== "event") return item.date === key;
  const { start, end } = itemBounds(item);
  return start < dayStart(key) + DAY && end > dayStart(key);
}
export function layoutDay(items: CalendarItem[], key: string, trackCount = 7) {
  const base = dayStart(key);
  const tracks = Math.max(1, Math.floor(trackCount));
  const segments = assignEventTracks(items).filter(item => item.kind === "event" && occursOnDay(item, key)).map(item => {
    const { start, end } = itemBounds(item);
    return { item, track: (item.track ?? 0) % tracks, start: Math.max(0, (start - base) / 60000), end: Math.min(1440, (end - base) / 60000), continuesBefore: start < base, continuesAfter: end > base + DAY };
  });
  return { segments, tracks };
}

// Allocate full intervals before clipping by day. Reuse observed occurrence lanes
// across adjacent pages, where earlier overlapping events may no longer be fetched.
export function assignEventTracks(items: CalendarItem[], remembered = new Map<string, number>()): CalendarItem[] {
  const intervals = items.filter(item => item.kind === "event").map(item => ({ item, ...itemBounds(item) }))
    .sort((a,b) => a.start - b.start || b.end - a.end || a.item.id.localeCompare(b.item.id));
  const lanes: { start: number; end: number }[][] = [];
  const assigned = new Map<string, number>();
  const fits = (track: number, value: { start: number; end: number }) => !lanes[track]?.some(other => value.start < other.end && value.end > other.start);
  const place = (value: typeof intervals[number], track: number) => {
    (lanes[track] ??= []).push(value);
    assigned.set(value.item.id, track);
    remembered.set(value.item.id, track);
  };
  // Reserve existing lanes first, so a newly fetched event cannot displace a
  // continuation already shown on the previous day.
  for (const value of intervals) {
    const track = value.item.track ?? remembered.get(value.item.id);
    if (track !== undefined && fits(track, value)) place(value, track);
  }
  for (const value of intervals) {
    if (assigned.has(value.item.id)) continue;
    let track = 0;
    while (!fits(track, value)) track++;
    place(value, track);
  }
  return items.map(item => item.kind === "event" ? { ...item, track: assigned.get(item.id) } : item);
}
