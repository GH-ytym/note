const test = require("node:test");
const assert = require("node:assert/strict");
const {
  calendarPairFromDayBounds,
  dayViewPairBounds,
  resizePeerBounds,
  translatePeerBounds,
  workspacePairBounds,
} = require("./workspace-layout.cjs");

const area = { x: 0, y: 0, width: 1920, height: 1040 };

test("calendar sits left of todo with a fixed gap at bottom-right", () => {
  const pair = workspacePairBounds(
    area,
    { width: 620, height: 380 },
    { width: 360, height: 150 },
    { edgeMargin: 14, gap: 14 },
  );

  assert.deepEqual(pair.calendar, { x: 912, y: 646, width: 620, height: 380 });
  assert.deepEqual(pair.day, { x: 1546, y: 646, width: 360, height: 150 });
  assert.equal(pair.day.x - pair.calendar.x - pair.calendar.width, 14);
});

test("moving either frame translates its peer by the same delta", () => {
  const peer = translatePeerBounds(
    { x: 100, y: 80, width: 360, height: 150 },
    { x: 145, y: 112, width: 360, height: 150 },
    { x: 474, y: 80, width: 620, height: 380 },
  );
  assert.deepEqual(peer, { x: 519, y: 112, width: 620, height: 380 });
});

test("resizing either frame preserves top alignment and the inner gap", () => {
  const day = resizePeerBounds(
    "calendar",
    { x: 80, y: 70, width: 700, height: 430 },
    { x: 714, y: 80, width: 360, height: 150 },
    14,
  );
  assert.deepEqual(day, { x: 794, y: 70, width: 360, height: 150 });

  const calendar = resizePeerBounds(
    "day",
    { x: 500, y: 64, width: 420, height: 300 },
    { x: 80, y: 70, width: 620, height: 380 },
    14,
  );
  assert.deepEqual(calendar, { x: -134, y: 64, width: 620, height: 380 });
});

test("a tall todo view shifts the whole pair on-screen without changing the gap", () => {
  const pair = dayViewPairBounds(
    area,
    { x: 912, y: 646, width: 620, height: 380 },
    { width: 360, height: 650 },
    { edgeMargin: 14, gap: 14 },
  );

  assert.equal(pair.calendar.y, 376);
  assert.equal(pair.day.y, 376);
  assert.equal(pair.day.x - pair.calendar.x - pair.calendar.width, 14);
  assert.equal(pair.day.y + pair.day.height, area.height - 14);
});

test("reopening the calendar restores it to the left of the current todo bounds", () => {
  const pair = calendarPairFromDayBounds(
    area,
    { x: 1546, y: 646, width: 360, height: 150 },
    { width: 620, height: 380 },
    { edgeMargin: 14, gap: 14 },
  );

  assert.deepEqual(pair.calendar, { x: 912, y: 646, width: 620, height: 380 });
  assert.deepEqual(pair.day, { x: 1546, y: 646, width: 360, height: 150 });
});
