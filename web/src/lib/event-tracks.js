// First-fit interval partitioning, with a fresh collision batch on overflow.
export function layoutEventTracks(segments, trackCount = 7) {
  const count = Math.max(1, Math.min(10, Math.round(Number(trackCount) || 7)));
  const ends = Array(count).fill(-Infinity);
  const ordered = [...segments].sort((a, b) =>
    a.start - b.start || b.end - a.end ||
    String(a.item.id).localeCompare(String(b.item.id)),
  );
  const placed = ordered.map(segment => {
    // Start times are sorted, so each track's last end represents all its
    // intervals in the current batch. Touching endpoints do not overlap.
    let track = ends.findIndex(end => end <= segment.start);
    if (track < 0) {
      ends.fill(-Infinity);
      track = 0;
    }
    ends[track] = segment.end;
    return { ...segment, track };
  });
  return { segments: placed, tracks: count };
}
