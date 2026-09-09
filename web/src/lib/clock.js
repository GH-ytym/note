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
