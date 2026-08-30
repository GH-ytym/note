const assert = require("node:assert/strict");
const test = require("node:test");

const {
  aboveAnchorBounds,
  bottomRightBounds,
  centeredBounds,
  leftOfBounds,
} = require("./window-layout.cjs");

const workArea = { x: 0, y: 0, width: 1920, height: 1040 };

test("day and reminder windows sit at the bottom-right work-area corner", () => {
  assert.deepEqual(
    bottomRightBounds(workArea, { width: 360, height: 150 }),
    { x: 1546, y: 876, width: 360, height: 150 },
  );
  assert.deepEqual(
    bottomRightBounds({ x: 1920, y: 40, width: 1280, height: 960 }, { width: 390, height: 250 }),
    { x: 2796, y: 736, width: 390, height: 250 },
  );
});

test("form windows sit directly above the day window", () => {
  const day = bottomRightBounds(workArea, { width: 360, height: 150 });
  assert.deepEqual(
    aboveAnchorBounds(workArea, { width: 340, height: 650 }, day),
    { x: 1566, y: 212, width: 340, height: 650 },
  );
});

test("calendar sits to the left and near the vertical center of the form column", () => {
  const day = bottomRightBounds(workArea, { width: 360, height: 150 });
  const form = aboveAnchorBounds(workArea, { width: 340, height: 650 }, day);
  assert.deepEqual(
    leftOfBounds(workArea, { width: 620, height: 380 }, form, { verticalOffset: 18 }),
    { x: 932, y: 365, width: 620, height: 380 },
  );
});

test("resizing a pinned window keeps its right and bottom margins", () => {
  const compact = bottomRightBounds(workArea, { width: 360, height: 150 });
  const expanded = bottomRightBounds(workArea, { width: compact.width, height: 310 });
  assert.equal(compact.x, expanded.x);
  assert.equal(compact.y + compact.height, expanded.y + expanded.height);
});

test("modal windows stay centered on their parent and inside the work area", () => {
  assert.deepEqual(
    centeredBounds(workArea, { width: 760, height: 560 }, { x: 1400, y: 300, width: 360, height: 650 }),
    { x: 1160, y: 345, width: 760, height: 560 },
  );
});
