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

export function stableClockTrack(item, trackCount) {
  const count = Math.max(1, Math.round(Number(trackCount) || 1));
  const key = String(item.eventId ?? item.id ?? "");
  let hash = 2166136261;
  for (let index = 0; index < key.length; index++) {
    hash ^= key.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0) % count;
}

export function clockTrackRadius(track, trackCount) {
  const count = Math.max(1, Math.round(Number(trackCount) || 1));
  if (count === 1) return 112;
  return 112 - track * (66 / (count - 1));
}
