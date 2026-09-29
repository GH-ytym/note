import test from 'node:test';
import assert from 'node:assert/strict';
import { AuthClient, APIError } from './auth-client.ts';

const user = (token = 'old') => ({ id: 12, name: 'alice', account: 'alice#12345', suffix: 12345, access_token: token, token_type: 'Bearer' });
const ok = () => new Response(JSON.stringify({ data: [] }), { status: 200 });

test('startup shares a single refresh; 401 is logged out while 503 stays retryable', async () => {
  let calls = 0;
  const client = new AuthClient(async () => { calls++; return user(); });
  await Promise.all([client.restore(), client.restore(), client.restore()]);
  assert.equal(calls, 1);
  const offline = new AuthClient(async () => { throw new APIError('offline', 503); });
  await assert.rejects(offline.restore(), { status: 503 });
  assert.equal(offline.initialized, false);
  const expired = new AuthClient(async () => { throw new APIError('expired', 401); });
  await expired.restore();
  assert.equal(expired.initialized, true);
  assert.equal(expired.user, null);
});

test('concurrent 401 requests share refresh and replay with the new bearer token', async () => {
  let refreshes = 0;
  const client = new AuthClient(async () => { refreshes++; return user('new'); });
  client.accept(user());
  const seen = [];
  const fetcher = async (_url, init) => {
    const token = init.headers.get('Authorization');
    seen.push(token);
    return token === 'Bearer old' ? new Response('{}', { status: 401 }) : ok();
  };
  await Promise.all([client.request('/todos', {}, fetcher), client.request('/calendar', {}, fetcher)]);
  assert.equal(refreshes, 1);
  assert.equal(seen.filter(token => token === 'Bearer new').length, 2);
});

test('a retried 401 does not loop; service failure does not erase the current login', async () => {
  const client = new AuthClient(async () => user('new'));
  client.accept(user());
  let sends = 0;
  await assert.rejects(client.request('/todos', {}, async () => { sends++; return new Response('{}', { status: 401 }); }), { status: 401 });
  assert.equal(sends, 2);
  const offline = new AuthClient(async () => { throw new APIError('offline', 503); });
  offline.accept(user());
  await assert.rejects(offline.logout(), { status: 503 });
  assert.equal(offline.user.id, 12);
});

test('logout waits for an in-flight refresh, and no later refresh can restore it', async () => {
  let finish;
  const actions = [];
  const client = new AuthClient(async action => {
    actions.push(action);
    if (action === 'refresh') return new Promise(resolve => { finish = resolve; });
    return null;
  });
  client.accept(user());
  const refreshing = client.refresh('old');
  await Promise.resolve();
  const exiting = client.logout();
  finish(user('new'));
  await Promise.all([refreshing, exiting]);
  assert.deepEqual(actions, ['refresh', 'logout']);
  assert.equal(client.user, null);
  await assert.rejects(client.refresh('old'), { status: 401 });
});

test('late business responses after logout are discarded', async () => {
  const client = new AuthClient(async () => null);
  client.accept(user());
  let finish;
  const request = client.request('/todos', {}, () => new Promise(resolve => { finish = resolve; }));
  await client.logout();
  finish(ok());
  await assert.rejects(request, { status: 401 });
});
