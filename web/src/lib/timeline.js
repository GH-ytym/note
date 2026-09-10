import { layoutEventTracks } from "./event-tracks.js";

// All calendar dates use the application's Asia/Shanghai time zone.
const DAY = 86400000;
export function dayStart(key) { return Date.parse(`${key}T00:00:00+08:00`); }
export function itemBounds(item) {
  const start = Date.parse(item.startsAt || `${item.date}T${item.time}:00+08:00`);
  const end = item.kind === "event" ? Date.parse(item.endsAt || `${item.endDate}T${item.endTime}:00+08:00`) : start;
  return { start, end };
}
export function occursOnDay(item, key) {
  if (item.kind !== "event") return item.date === key;
  const { start, end } = itemBounds(item);
  return start < dayStart(key) + DAY && end > dayStart(key);
}
export function layoutDay(items, key, trackCount = 7) {
  const base = dayStart(key);
  const segments = items.filter(item => item.kind === "event" && occursOnDay(item, key)).map(item => {
    const { start, end } = itemBounds(item);
    return { item, start: Math.max(0, (start - base) / 60000), end: Math.min(1440, (end - base) / 60000), continuesBefore: start < base, continuesAfter: end > base + DAY };
  });
  return layoutEventTracks(segments, trackCount);
}
