export function clockPoint(minutes, radius) {
  const angle = minutes / 1440 * Math.PI * 2 - Math.PI / 2;
  return { x: 180 + Math.cos(angle) * radius, y: 180 + Math.sin(angle) * radius };
}
export function clockMinute(x, y) {
  return ((Math.atan2(y - 180, x - 180) + Math.PI / 2 + Math.PI * 2) % (Math.PI * 2)) / (Math.PI * 2) * 1440;
}
export function minuteDelta(previous, next) {
  let delta = next - previous;
  if (delta > 720) delta -= 1440;
  if (delta < -720) delta += 1440;
  return delta;
}
export function clockArc(start, end, radius) {
  const a = clockPoint(start, radius), b = clockPoint(Math.min(end, start + 1439.99), radius);
  return `M ${a.x} ${a.y} A ${radius} ${radius} 0 ${end - start > 720 ? 1 : 0} 1 ${b.x} ${b.y}`;
}

export function clockTrackRadius(track) {
  return 116 - track * 5.5;
}

export function layoutTodoTracks(items, date, trackCount = 7) {
  const count = Math.max(1, Math.min(10, Number(trackCount) || 7));
  const tracks = Array.from({ length: count }, () => []);
  return items.filter(item => item.kind === "todo" && item.date === date)
    .map(item => ({ item, minute: Number(item.time.slice(0, 2)) * 60 + Number(item.time.slice(3, 5)) }))
    .sort((a, b) => a.minute - b.minute || String(a.item.id).localeCompare(String(b.item.id)))
    .map(segment => {
      let track = tracks.findIndex(placed => placed.every(minute => {
        const distance = Math.abs(minute - segment.minute);
        return Math.min(distance, 1440 - distance) >= 20;
      }));
      if (track < 0) { tracks.forEach(placed => placed.length = 0); track = 0; }
      tracks[track].push(segment.minute);
      return { ...segment, track };
    });
}
