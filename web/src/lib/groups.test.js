import test from 'node:test';
import assert from 'node:assert/strict';
import { invitationValue, parseInvite } from './groups.ts';

test('invite card works locally, while deployed frontend shares a link', () => {
  const local = invitationValue(12, 'A2BC34', 'http://127.0.0.1:8080');
  assert.equal(local.includes('127.0.0.1'), false);
  assert.deepEqual(parseInvite(local), { groupID: 12, code: 'A2BC34' });
  const publicLink = invitationValue(12, 'A2BC34', 'https://note.example.com');
  assert.ok(publicLink.startsWith('https://note.example.com/?'));
  assert.deepEqual(parseInvite(publicLink), { groupID: 12, code: 'A2BC34' });
  assert.equal(parseInvite('https://note.example.com/?invite_group=-1&code=A2BC34'), null);
  assert.equal(parseInvite('https://note.example.com/?invite_group=12&code=short'), null);
});
