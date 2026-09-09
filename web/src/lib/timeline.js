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
export function layoutDay(items, key, maxTracks = Infinity) {
  const base = dayStart(key);
  const segments = items.filter(item => item.kind === "event" && occursOnDay(item, key)).map(item => {
    const { start, end } = itemBounds(item);
    return { item, start: Math.max(0, (start - base) / 60000), end: Math.min(1440, (end - base) / 60000), continuesBefore: start < base, continuesAfter: end > base + DAY };
  }).sort((a, b) => a.start - b.start || b.end - a.end || a.item.id.localeCompare(b.item.id));
  const trackEnds = [];
  for (const segment of segments) {
    let track = trackEnds.findIndex(end => end <= segment.start);
    if (track < 0) track = trackEnds.length < maxTracks ? trackEnds.length : trackEnds.indexOf(Math.min(...trackEnds));
    segment.track = track;
    trackEnds[track] = Math.max(trackEnds[track] || 0, segment.end);
  }
  // When four week tracks are full, retain separate I bars inside the shared track.
  for (let track = 0; track < trackEnds.length; track++) {
    let group = [], until = -1;
    const finish = () => group.forEach((segment, slot) => { segment.slot = slot; segment.slots = group.length; });
    for (const segment of segments.filter(segment => segment.track === track)) {
      if (segment.start >= until) { finish(); group = []; }
      group.push(segment);
      until = Math.max(until, segment.end);
    }
    finish();
  }
  return { segments, tracks: Math.max(1, trackEnds.length) };
}
