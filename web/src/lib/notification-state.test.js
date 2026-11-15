import test from 'node:test';
import assert from 'node:assert/strict';
import {applyChange,mergePage} from './notification-state.ts';
const page={revision:'9007199254740992',unread_count:2,next:100,items:[{id:103,read_at:null},{id:100,read_at:null}]};
test('precise versions, read changes, removal and loaded range',()=>{
 const result=applyChange(page,{revision:'9007199254740993',unread_count:1,upserts:[{id:104},{id:90}],removed_ids:[100],read_ids:[103],read_at:'today'});
 assert.deepEqual(result.items.map(x=>x.id),[104,103]);assert.equal(result.items[1].read_at,'today');
 assert.equal(result.unread_count,1);assert.equal(result.next,100);
 assert.equal(applyChange(result,{revision:page.revision}),result);
 assert.throws(()=>applyChange(page,{revision:'9007199254740994'}));
 assert.equal(page.items[0].read_at,null);
});
test('page revisions must agree, duplicate records keep newer local data',()=>{
 const result=mergePage(page,{...page,next:0,items:[{id:100,read_at:'stale'},{id:98}]});
 assert.deepEqual(result.items.map(x=>x.id),[103,100,98]);assert.equal(result.items[1].read_at,null);
 assert.throws(()=>mergePage(page,{...page,revision:'1'}));
});
