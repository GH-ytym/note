import type { CalendarItem } from "../types";

// Todos are points, so each timestamp gets its own bounded set of lanes.
export function layoutTimelineTodos(items: CalendarItem[], date: string, trackCount: number) {
  const tracks = Math.max(1, Math.floor(trackCount));
  const used = new Map<string, number>();
  return items.filter(item => item.kind === "todo" && item.date === date)
    .sort((a, b) => a.time.localeCompare(b.time) || a.id.localeCompare(b.id))
    .flatMap(item => {
      const track = used.get(item.time) ?? 0;
      used.set(item.time, track + 1);
      const [hour, minute] = item.time.split(":").map(Number);
      return track < tracks ? [{ item, track, minute: hour * 60 + minute }] : [];
    });
}
