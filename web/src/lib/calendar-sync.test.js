import test from 'node:test';
import assert from 'node:assert/strict';
import { createCalendarSync } from './calendar-sync.ts';
import { parseCalendarChange } from './calendar-change.ts';
import { consumeNotifications } from './notification-stream.ts';

const flush = async () => { for (let i = 0; i < 15; i++) await Promise.resolve(); };
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };

test('calendar SSE is delivered alongside notifications without changing their payload', async () => {
  const change = { group_id: 3, kind: 'todo', id: 42, action: 'remove' };
  const data = `event:notification.snapshot\ndata:{"revision":"12"}\n\nevent:cldr\ndata:${JSON.stringify(change)}\n\nevent:notification.change\ndata:{"revision":"13"}\n\n`;
  const body = new ReadableStream({ start(c) { for (const b of new TextEncoder().encode(data)) c.enqueue(Uint8Array.of(b)); c.close(); } });
  const events = [];
  await consumeNotifications(body, event => events.push(event));
  assert.deepEqual(events.map(e => e.name), ['notification.snapshot', 'cldr', 'notification.change']);
  assert.deepEqual(parseCalendarChange(events[1].data), change);
  assert.equal(events[2].data.revision, '13');
  for (const bad of [null, {}, {...change, group_id: 0}, {...change, id: -1}, {...change, kind: 'user'}, {...change, action: 'delete'}]) {
    assert.throws(() => parseCalendarChange(bad));
  }
});

test('requests coalesce and changes during loading cause one follow-up, never parallel queries', async t => {
  const loads = [], applied = [];
  const sync = createCalendarSync({ load: () => { const d = deferred(); loads.push(d); return d.promise; }, apply: v => applied.push(v), fail: assert.fail });
  t.after(() => sync.stop());
  sync.setActive(true);
  const done = sync.request(); sync.request();
  await flush();
  assert.equal(loads.length, 1);
  sync.request(); sync.request();
  assert.equal(loads.length, 1);
  loads[0].resolve(['old']);
  await flush();
  assert.equal(loads.length, 2);
  loads[1].resolve(['new']);
  await done;
  assert.deepEqual(applied, [['old'], ['new']]);
});

test('polls 60s after completion, pauses while inactive and refreshes on activation', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0;
  const sync = createCalendarSync({ load: async () => ++calls, apply() {}, fail: assert.fail });
  t.after(() => sync.stop());
  sync.setActive(true); await flush();
  assert.equal(calls, 1);
  t.mock.timers.tick(59_999); await flush(); assert.equal(calls, 1);
  t.mock.timers.tick(1); await flush(); assert.equal(calls, 2);
  sync.setActive(false);
  t.mock.timers.tick(180_000); await sync.request(); assert.equal(calls, 2);
  sync.setActive(true); await flush(); assert.equal(calls, 3);
  sync.stop(); t.mock.timers.tick(180_000); await flush(); assert.equal(calls, 3);
});

test('failed refresh retains data and next poll replaces it, including missed deletions', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0, items = [], failures = 0;
  const sync = createCalendarSync({
    load: async () => { calls++; if (calls === 2) throw new Error('offline'); return calls === 1 ? [1, 2] : [2]; },
    apply: value => { items = value; }, fail: () => failures++,
  });
  t.after(() => sync.stop());
  sync.setActive(true); await flush(); assert.deepEqual(items, [1, 2]);
  t.mock.timers.tick(60_000); await flush(); assert.equal(failures, 1); assert.deepEqual(items, [1, 2]);
  t.mock.timers.tick(60_000); await flush(); assert.deepEqual(items, [2]);
});

test('range disposal aborts pending request and ignores its late response', async () => {
  const d = deferred(); let signal, applied = 0, errors = 0, settled = 0;
  const sync = createCalendarSync({ load: s => { signal = s; return d.promise; }, apply: () => applied++, fail: () => errors++, settled: () => settled++ });
  sync.setActive(true); await flush();
  sync.stop(); assert.equal(signal.aborted, true);
  d.resolve('old group'); await flush();
  assert.deepEqual([applied, errors, settled], [0, 0, 0]);
});

test('slow query does not accumulate periodic requests', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const d = deferred(); let calls = 0;
  const sync = createCalendarSync({ load: () => { calls++; return d.promise; }, apply() {}, fail: assert.fail });
  t.after(() => sync.stop());
  sync.setActive(true); await flush();
  t.mock.timers.tick(180_000); await flush(); assert.equal(calls, 1);
  d.resolve([]); await flush();
  t.mock.timers.tick(59_999); await flush(); assert.equal(calls, 1);
  t.mock.timers.tick(1); await flush(); assert.equal(calls, 2);
});
