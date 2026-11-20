import test from 'node:test';
import assert from 'node:assert/strict';
import { createReviewer, notificationAction } from './notification-review.ts';

const card = {id:42,join_request_id:20,type:'join_requested',request_status:'pending',read_at:null};

test('review uses request ID, blocks duplicate submissions across views and leaves counts to sync', async () => {
  let resolve, calls=0, synced=0, states;
  const review=createReviewer((path, options)=>{
    calls++;
    assert.equal(path,'/groups/join-requests/20/review');
    assert.equal(options.method,'POST');
    assert.deepEqual(JSON.parse(options.body),{decision:'accept'});
    return new Promise(done=>{resolve=done;});
  }, value=>{states=value;}, ()=>{synced++;});
  const first=review(card,'accept');
  assert.equal(states[20].busy,true);
  await review({...card},'reject');
  assert.equal(calls,1);
  resolve(); await first;
  assert.equal(states[20].done,'accepted');
  assert.equal(states[20].busy,false);
  assert.equal(synced,1);
  await review(card,'reject');
  assert.equal(calls,1);
  assert.equal(card.read_at,null);
  assert.equal(card.request_status,'pending');
});

test('failure shows server error and allows retry; rejection is a successful decision', async () => {
  let calls=0, states, synced=0;
  const review=createReviewer(async (_,options)=>{
    assert.equal(JSON.parse(options.body).decision,'reject');
    if (++calls===1) throw new Error('连接失败');
  }, value=>{states=value;},()=>{synced++;});
  await review(card,'reject');
  assert.equal(states[20].error,'连接失败');
  assert.equal(states[20].busy,false);
  assert.equal(synced,0);
  await review(card,'reject');
  assert.equal(states[20].done,'rejected');
  assert.equal(states[20].error,'');
  assert.equal(synced,1);
});

test('stale requests resync while permission failures preserve the error', async () => {
  for (const status of [403,404,409]) {
    let states, synced=0;
    const review=createReviewer(async()=>{throw Object.assign(new Error('申请状态已变化'),{status});}, value=>{states=value;},()=>{synced++;});
    await review(card,'accept');
    assert.equal(states[20].error,'申请状态已变化');
    assert.equal(states[20].done,undefined);
    assert.equal(synced,status===403?0:1);
  }
});

test('completed cards, result cards and cards missing request IDs cannot be reviewed', async () => {
  const review=createReviewer(async()=>assert.fail('unexpected request'),()=>assert.fail('unexpected state'),()=>assert.fail('unexpected sync'));
  await review({...card,request_status:'accepted'},'accept');
  await review({...card,type:'join_reviewed'},'accept');
  await review({...card,join_request_id:undefined},'accept');
});

test('review result text describes the decision rather than the owner joining', () => {
  assert.equal(notificationAction({...card,type:'join_reviewed',request_status:'accepted'}),'已通过你的入群申请：');
  assert.equal(notificationAction({...card,type:'join_reviewed',request_status:'rejected'}),'已拒绝你的入群申请：');
});
