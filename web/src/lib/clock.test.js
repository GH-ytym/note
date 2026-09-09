import test from 'node:test';
import assert from 'node:assert/strict';
import {
  clockMinute,
  clockPoint,
  minuteDelta,
  clockArc,
  clockTrackRadius,
  stableClockTrack,
} from "./clock.js";
test('24h clock maps cardinal points and arbitrary times',()=>{
 for(const minute of [0,120,360,720,1080,1435]){const p=clockPoint(minute,140);assert.ok(Math.abs(clockMinute(p.x,p.y)-minute)<0.00001);}
});
test('dragging through midnight keeps a small continuous delta',()=>{
 assert.equal(minuteDelta(1435,5),10);assert.equal(minuteDelta(5,1435),-10);
});
test('full-day arcs remain drawable',()=>{
 assert.match(clockArc(0,1440,100),/A 100 100 0 1 1/);
 assert.doesNotMatch(clockArc(0,1440,100),/NaN/);
});
test("an event keeps its clock track when its time changes", () => {
  const before = { id: "event-12-old", eventId: 12, time: "09:00" };
  const after = { id: "event-12-new", eventId: 12, time: "18:30" };
  assert.equal(stableClockTrack(before, 7), stableClockTrack(after, 7));
  assert.equal(clockTrackRadius(0, 7), 112);
  assert.equal(clockTrackRadius(6, 7), 46);
});
