const assert = require("node:assert/strict");
const test = require("node:test");

const {
  ReminderScheduler,
  dateKeyAt,
  isReminderCandidate,
  nextDateKey,
  occurrenceKey,
} = require("./reminder-scheduler.cjs");

function fakeTimers() {
  const timers = [];
  return {
    timers,
    setTimer(callback, delay) {
      const timer = { callback, delay, cleared: false };
      timers.push(timer);
      return timer;
    },
    clearTimer(timer) {
      timer.cleared = true;
    },
  };
}

test("date helpers use the application's Shanghai calendar day", () => {
  const timestamp = Date.parse("2026-08-29T16:30:00Z");
  assert.equal(dateKeyAt(timestamp), "2026-08-30");
  assert.equal(nextDateKey("2026-12-31"), "2027-01-01");
});

test("only future, incomplete occurrences with a reminder are candidates", () => {
  const now = Date.parse("2026-08-29T01:00:00Z");
  const base = {
    todo_id: 7,
    occurs_at: "2026-08-29T09:05:00+08:00",
    notify_mode: "popup",
    all_done: false,
    occurrence_done: false,
  };

  assert.equal(isReminderCandidate(base, now), true);
  assert.equal(isReminderCandidate({ ...base, occurs_at: "2026-08-29T08:55:00+08:00" }, now), false);
  assert.equal(isReminderCandidate({ ...base, notify_mode: "none" }, now), false);
  assert.equal(isReminderCandidate({ ...base, all_done: true }, now), false);
  assert.equal(isReminderCandidate({ ...base, occurrence_done: true }, now), false);
  assert.equal(occurrenceKey(base), "7:2026-08-29T01:05:00.000Z");
});

test("scheduler loads today, schedules future occurrences and fires each once", async () => {
  let now = Date.parse("2026-08-29T01:00:00Z");
  const clock = fakeTimers();
  const fired = [];
  const requestedRanges = [];
  const future = {
    todo_id: 7,
    content: "喝水",
    occurs_at: "2026-08-29T09:05:00+08:00",
    notify_mode: "popup",
    all_done: false,
    occurrence_done: false,
  };
  const scheduler = new ReminderScheduler({
    now: () => now,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    loadOccurrences: async (from, to) => {
      requestedRanges.push([from, to]);
      return [
        future,
        { ...future, todo_id: 8, occurs_at: "2026-08-29T08:30:00+08:00" },
        { ...future, todo_id: 9, occurrence_done: true },
      ];
    },
    onReminder: (item) => fired.push(item.todo_id),
  });

  await scheduler.start();

  assert.deepEqual(requestedRanges, [["2026-08-29", "2026-08-30"]]);
  assert.equal(scheduler.scheduledCount, 1);
  const occurrenceTimer = clock.timers.find((timer) => timer.delay === 5 * 60_000);
  assert.ok(occurrenceTimer);

  now += 10 * 60_000;
  occurrenceTimer.callback();
  occurrenceTimer.callback();
  assert.deepEqual(fired, [7]);

  await scheduler.refresh();
  assert.equal(scheduler.scheduledCount, 0);
  scheduler.stop();
});

test("refresh cancels old timers before arranging changed data", async () => {
  const now = Date.parse("2026-08-29T01:00:00Z");
  const clock = fakeTimers();
  let content = "旧时间";
  const scheduler = new ReminderScheduler({
    now: () => now,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    loadOccurrences: async () => [{
      todo_id: 3,
      content,
      occurs_at: content === "旧时间"
        ? "2026-08-29T10:00:00+08:00"
        : "2026-08-29T11:00:00+08:00",
      notify_mode: "silent",
      all_done: false,
      occurrence_done: false,
    }],
    onReminder: () => {},
  });

  await scheduler.start();
  const oldOccurrenceTimer = clock.timers.find((timer) => timer.delay === 60 * 60_000);
  assert.ok(oldOccurrenceTimer);

  content = "新时间";
  await scheduler.refresh();
  assert.equal(oldOccurrenceTimer.cleared, true);
  assert.equal(scheduler.scheduledCount, 1);
  assert.ok(clock.timers.some((timer) => timer.delay === 2 * 60 * 60_000));
  scheduler.stop();
});
