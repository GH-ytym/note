import test from 'node:test';
import assert from 'node:assert/strict';
import { consumeNotifications } from './notification-stream.ts';
import { AuthClient } from './auth-client.ts';

test('SSE handles chunk boundaries, heartbeats, CRLF and multiple events', async () => {
  const encoder = new TextEncoder();
  const chunks = [': connected\n\neve', 'nt: notification.snapshot\r', '\ndata: {"name":"用户"}\r\n\r', '\n: ping\n\nevent: notification.change\ndata: {}\n\nevent: unknown\ndata: {}\n\n', 'event: notification.snapshot\ndata: incomplete'];
  const events = [];
  const stream = new ReadableStream({ start(controller) {
    for (const chunk of chunks) {
      // Split even multibyte Chinese characters across reads.
      for (const byte of encoder.encode(chunk)) controller.enqueue(Uint8Array.of(byte));
    }
    controller.close();
  } });
  await consumeNotifications(stream, event => events.push(event));
  assert.equal(events.length, 2); assert.deepEqual(events[0].data, {name:"用户"});
});

test('streaming response refreshes rejected token without consuming SSE body', async () => {
  const user = token => ({ id: 1, name: 'test', suffix: 1, account: 'test#00001', access_token: token, token_type: 'Bearer' });
  let refreshes = 0;
  const client = new AuthClient(async () => { refreshes++; return user('new'); });
  client.accept(user('old'));
  const tokens = [];
  const response = await client.response('/notifications/stream', {}, async (_url, init) => {
    const token = init.headers.get('Authorization');
    tokens.push(token);
    return token === 'Bearer old' ? new Response('{}', { status: 401 })
      : new Response(': connected\n\n', { headers: { 'Content-Type': 'text/event-stream' } });
  });
  assert.equal(refreshes, 1);
  assert.deepEqual(tokens, ['Bearer old', 'Bearer new']);
  assert.equal(response.bodyUsed, false);
  assert.equal(await response.text(), ': connected\n\n');
});
