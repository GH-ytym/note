import test from "node:test";
import assert from "node:assert/strict";
import { occursOnDay, layoutDay, assignEventTracks } from "./timeline.ts";

const event = (id, date, time, endDate, endTime) => ({ id, kind: "event", date, time, endDate, endTime });
test("cross-month events cover each intersected day but exclude the midnight end", () => {
  const item = event("1", "2026-01-30", "22:00", "2026-02-03", "00:00");
  assert.equal(occursOnDay(item, "2026-01-29"), false);
  for (const date of ["2026-01-30", "2026-01-31", "2026-02-01", "2026-02-02"]) assert.equal(occursOnDay(item, date), true);
  assert.equal(occursOnDay(item, "2026-02-03"), false);
  const { segments } = layoutDay([item], "2026-02-01");
  assert.equal(segments[0].start, 0);
  assert.equal(segments[0].end, 1440);
  assert.equal(segments[0].continuesBefore, true);
  assert.equal(segments[0].continuesAfter, true);
});
test("configured lanes stay fixed even when only two are occupied", () => {
  const items = [event("a", "2026-09-09", "09:00", "2026-09-09", "11:00"), event("b", "2026-09-09", "10:00", "2026-09-09", "12:00")];
  const result = layoutDay(items, "2026-09-09", 7);
  assert.equal(result.tracks, 7);
  assert.deepEqual(result.segments.map(item => item.track), [0, 1]);
});
test("all views wrap at the configured limit without dropping events", () => {
  const items = Array.from({ length: 6 }, (_, i) => event(String(i), "2026-09-09", "09:00", "2026-09-09", "11:00"));
  const week = layoutDay(items, "2026-09-09", 4);
  assert.equal(week.segments.length, 6);
  assert.equal(week.tracks, 4);
  assert.deepEqual(week.segments.map(item => item.track), [0, 1, 2, 3, 0, 1]);
  assert.equal(layoutDay(items, "2026-09-09", 7).tracks, 7);
});
test("adjacent intervals reuse tracks and todo stays on its own date", () => {
  const items = [event("a", "2026-09-09", "09:00", "2026-09-09", "10:00"), event("b", "2026-09-09", "10:00", "2026-09-09", "11:00")];
  assert.deepEqual(layoutDay(items, "2026-09-09", 7).segments.map(item => item.track), [0, 0]);
  assert.equal(occursOnDay({ kind: "todo", date: "2026-09-09" }, "2026-09-10"), false);
});

test("multi-day events keep lanes when earlier events disappear on subsequent days", () => {
  const items = [event("early", "2026-09-09", "08:00", "2026-09-09", "12:00"), event("long", "2026-09-09", "09:00", "2026-09-12", "12:00")];
  const memory = new Map();
  const first = assignEventTracks(items, memory);
  assert.equal(first.find(item => item.id === "long").track, 1);
  for (const key of ["2026-09-09", "2026-09-10", "2026-09-11", "2026-09-12"]) {
    assert.equal(layoutDay(first, key).segments.find(segment => segment.item.id === "long").track, 1);
  }
  const nextPage = assignEventTracks([items[1]], memory);
  assert.equal(layoutDay(nextPage, "2026-09-10").segments[0].track, 1);
});

test("new overlapping events do not displace an existing continuation", () => {
  const long = event("long", "2026-09-09", "09:00", "2026-09-12", "12:00");
  const memory = new Map();
  assignEventTracks([long], memory);
  const earlier = event("earlier", "2026-09-09", "08:00", "2026-09-10", "12:00");
  const result = assignEventTracks([earlier, long], memory);
  assert.equal(result.find(item => item.id === "long").track, 0);
  assert.equal(result.find(item => item.id === "earlier").track, 1);
});
