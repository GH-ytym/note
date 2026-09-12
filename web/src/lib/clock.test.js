import test from 'node:test';
import assert from 'node:assert/strict';
import {
  clockMinute,
  clockPoint,
  minuteDelta,
  clockArc,
  clockTrackRadius,
  layoutTodoTracks,
} from "./clock.ts";
test('24h clock maps cardinal points and arbitrary times',()=>{
 for(const minute of [0,120,360,720,1080,1435]){const p=clockPoint(minute,140);assert.ok(Math.abs(clockMinute(p.x,p.y)-minute)<0.00001);}
});
test("todo pins separate coincident and midnight-adjacent times into non-overlapping outer tracks", () => {
  const items = ["00:00", "00:00", "00:10", "12:00", "23:55"].map((time, id)=>({id:String(id),kind:"todo",date:"2026-09-10",time}));
  const pins = layoutTodoTracks(items,"2026-09-10",7);
  assert.deepEqual(pins.map(pin=>pin.track),[0,1,2,0,3]);
  assert.equal(layoutTodoTracks(items,"2026-09-11",7).length,0);
});
test('dragging through midnight keeps a small continuous delta',()=>{
 assert.equal(minuteDelta(1435,5),10);assert.equal(minuteDelta(5,1435),-10);
});
test('full-day arcs remain drawable',()=>{
 assert.match(clockArc(0,1440,100),/A 100 100 0 1 1/);
 assert.doesNotMatch(clockArc(0,1440,100),/NaN/);
});
test("clock keeps the outer radius and half spacing", () => {
  assert.equal(clockTrackRadius(0), 116);
  assert.equal(clockTrackRadius(1), 110.5);
  assert.equal(clockTrackRadius(6), 83);
});
