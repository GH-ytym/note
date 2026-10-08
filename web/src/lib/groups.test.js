import test from 'node:test';
import assert from 'node:assert/strict';
import { invitationValue, parseInvite } from './groups.ts';
test('fixed group code parses from direct input, cards and links without internal ID', () => {
  assert.deepEqual(parseInvite(' a2bc34 '), { code: 'A2BC34' });
  for (const origin of ['http://127.0.0.1:8080', 'https://note.example.com']) {
    const value = invitationValue('A2BC34', origin);
    assert.deepEqual(parseInvite(value), { code: 'A2BC34' });
    assert.equal(value.includes('invite_group'), false);
  }
  assert.equal(parseInvite('javascript:alert(1)'), null);
  assert.equal(parseInvite('https://note.example.com/?code=short'), null);
  assert.equal(parseInvite('ABC1234'), null);
});
