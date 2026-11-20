import test from 'node:test';
import assert from 'node:assert/strict';
import { AuthClient } from './auth-client.ts';

test('remote API and SSE retain the deployment prefix and bearer token', async () => {
  const client = new AuthClient(async () => null, '/p/note/api');
  client.accept({id:1,name:'test',suffix:1,account:'test',access_token:'test-token',token_type:'Bearer'});
  for (const path of ['/groups', '/notifications/stream']) {
    await client.response(path, {}, async (url, options) => {
      assert.equal(url, '/p/note/api' + path);
      assert.equal(options.headers.get('Authorization'), 'Bearer test-token');
      assert.equal(options.credentials, 'include');
      return new Response('{}', {status:200});
    });
  }
});
