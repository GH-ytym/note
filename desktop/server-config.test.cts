import test = require("node:test");
import assert = require("node:assert/strict");
import { serverURL, apiURL } from "./server-config.cjs";

test("server URL keeps login, API and SSE on one HTTPS origin", () => {
  assert.equal(serverURL(" https://note.example.com/ "), "https://note.example.com");
  assert.equal(serverURL("http://127.0.0.1:8080"), "http://127.0.0.1:8080");
  assert.equal(apiURL(serverURL("https://gis.epawse.xyz/p/note/"), "auth/login"), "https://gis.epawse.xyz/p/note/api/auth/login");
  assert.equal(apiURL(serverURL("https://gis.epawse.xyz/p/note/"), "notifications/stream"), "https://gis.epawse.xyz/p/note/api/notifications/stream");
  for (const url of [undefined, "", "http://172.16.120.29", "file:///tmp/a", "https://u:p@note.example.com", "https://note.example.com/p/note", "https://note.example.com/?url=x", "https://note.example.com/#x"]) {
    assert.throws(() => serverURL(url));
  }
});
