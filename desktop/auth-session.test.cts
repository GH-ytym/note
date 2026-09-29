import test = require("node:test");
import assert = require("node:assert/strict");
import { AuthSession } from "./auth-session.cjs";
import type { AuthResult, AuthUser } from "./contracts.cjs";
const user: AuthUser = { id: 1, name: "alice", account: "alice#12345", suffix: 12345, access_token: "old", token_type: "Bearer" };

test("windows share startup restore and concurrent token refresh", async () => {
  let calls = 0;
  const auth = new AuthSession(async () => ({ user: { ...user, access_token: String(++calls) }, status: 200 }), () => {});
  await Promise.all([auth.run({ action: "restore" }), auth.run({ action: "restore" })]);
  assert.equal(calls, 1);
  await Promise.all([auth.run({ action: "refresh", token: "1" }), auth.run({ action: "refresh", token: "1" })]);
  assert.equal(calls, 2);
});

test("logout failure preserves login; successful logout prevents stale refresh", async () => {
  let fail = true;
  const auth = new AuthSession(async action => action === "logout" ? { user: null, status: fail ? 503 : 204, error: fail ? "offline" : undefined } : { user, status: 200 }, () => {});
  await auth.run({ action: "restore" });
  await auth.run({ action: "logout" });
  assert.equal(auth.user, user);
  fail = false;
  await auth.run({ action: "logout" });
  const result = await auth.run({ action: "refresh", token: "old" });
  assert.equal(result.status, 401);
  assert.equal(auth.user, null);
});

test("logout serializes after an active rotation", async () => {
  let resolve!: (value: AuthResult) => void;
  const auth = new AuthSession(async action => action === "logout" ? { user: null, status: 204 } : new Promise(done => { resolve = done; }), () => {});
  const restore = auth.run({ action: "restore" });
  await Promise.resolve();
  const logout = auth.run({ action: "logout" });
  resolve({ user, status: 200 });
  await Promise.all([restore, logout]);
  assert.equal(auth.user, null);
});
