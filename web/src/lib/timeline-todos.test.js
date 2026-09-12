import test from "node:test";
import assert from "node:assert/strict";
import { layoutTimelineTodos } from "./timeline-todos.ts";
import { dragTimeLabel } from "./drag-time.ts";

const todo = (id, time = "09:00", date = "2026-09-12") => ({id, kind:"todo", date, time});
test("coincident todos use each lane once and hide overflow without wrapping", () => {
  const items = Array.from({length:8}, (_, i) => todo(String(i)));
  const result = layoutTimelineTodos(items, "2026-09-12", 3);
  assert.deepEqual(result.map(x => [x.item.id, x.track]), [["0",0],["1",1],["2",2]]);
  assert.deepEqual(layoutTimelineTodos(items.reverse(), "2026-09-12", 3), result);
});
test("each timestamp and date gets independent lane capacity", () => {
  const items = [todo("a"),todo("b"),todo("c","09:05"),todo("d","09:00","2026-09-13"),{...todo("event"),kind:"event"}];
  assert.deepEqual(layoutTimelineTodos(items,"2026-09-12",1).map(x => [x.item.id,x.track,x.minute]), [["a",0,540],["c",0,545]]);
});
test("drag labels show destination date and time across midnight for either endpoint", () => {
  assert.equal(dragTimeLabel(todo("a","23:55"),"start",10),"2026-09-13 00:05");
  assert.equal(dragTimeLabel(todo("a","00:05"),"start",-10),"2026-09-11 23:55");
  const event = {...todo("e"),kind:"event",endsAt:"2026-09-13T10:00:00+08:00"};
  assert.equal(dragTimeLabel(event,"end",30),"2026-09-13 10:30");
});
