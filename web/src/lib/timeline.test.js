import test from "node:test";
import assert from "node:assert/strict";
import { occursOnDay, layoutDay } from "./timeline.js";

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
test("week caps lanes at four without discarding collisions; day keeps all lanes", () => {
  const items = Array.from({ length: 6 }, (_, i) => event(String(i), "2026-09-09", "09:00", "2026-09-09", "11:00"));
  const week = layoutDay(items, "2026-09-09", 4);
  assert.equal(week.segments.length, 6);
  assert.equal(week.tracks, 4);
  assert.deepEqual(week.segments.slice(0, 4).map(item => item.track), [0, 1, 2, 3]);
  assert.equal(layoutDay(items, "2026-09-09").tracks, 6);
});
test("adjacent intervals reuse tracks and todo stays on its own date", () => {
  const items = [event("a", "2026-09-09", "09:00", "2026-09-09", "10:00"), event("b", "2026-09-09", "10:00", "2026-09-09", "11:00")];
  assert.equal(layoutDay(items, "2026-09-09").tracks, 1);
  assert.equal(occursOnDay({ kind: "todo", date: "2026-09-09" }, "2026-09-10"), false);
});
