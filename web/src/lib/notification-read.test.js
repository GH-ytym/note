import test from 'node:test';
import assert from 'node:assert/strict';
import { createReadMarker } from './notification-read.ts';

test('viewing a result marks only that card, coalesces duplicates and keeps counts server-owned', async () => {
  let finish;
  const calls = [];
  const mark = createReadMarker(ids => { calls.push(ids); return new Promise(resolve => { finish = resolve; }); });
  const card = {id:10, read_at:null, type:'join_reviewed'};
  const first = mark(card), second = mark(card);
  assert.equal(first, second);
  await Promise.resolve();
  assert.deepEqual(calls, [[10]]);
  finish(); await first;
  await mark(card);
  await mark({id:11, read_at:'2026-11-21T12:00:00Z'});
  assert.deepEqual(calls, [[10]]);
  assert.equal(card.read_at, null);
});

test('failed read can be retried, including pending applications without submitting a decision', async () => {
  let calls = 0;
  const mark = createReadMarker(async ids => { assert.deepEqual(ids,[9]); if (++calls === 1) throw new Error('offline'); });
  const card = {id:9, read_at:null, type:'join_requested', request_status:'pending'};
  await assert.rejects(mark(card), /offline/);
  await mark(card);
  assert.equal(calls,2);
  assert.equal(card.request_status,'pending');
});
