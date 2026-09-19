import test = require("node:test");
import assert = require("node:assert/strict");
import fs = require("node:fs");
import path = require("node:path");
import vm = require("node:vm");
import type { NoteDesktop } from "./contracts.cjs";

test("compiled sandbox preload exposes the typed bridge without runtime local imports", async () => {
  let bridge: NoteDesktop | undefined;
  const calls: {channel: string; payload: unknown}[] = [];
  const listeners = new Map<string, (event: unknown, payload: unknown) => void>();
  const electron = {
    contextBridge: { exposeInMainWorld(name: string, value: NoteDesktop) {
      assert.equal(name, "noteDesktop"); bridge = value;
    } },
    ipcRenderer: {
      invoke(channel: string, payload?: unknown) { calls.push({channel, payload}); return Promise.resolve(); },
      on(channel: string, listener: (event: unknown, payload: unknown) => void) { listeners.set(channel, listener); },
      removeListener(channel: string, listener: (event: unknown, payload: unknown) => void) {
        assert.equal(listeners.get(channel), listener); listeners.delete(channel);
      },
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, "preload.cjs"), "utf8"), {
    exports: {}, require(name: string) { assert.equal(name, "electron"); return electron; },
  });
  assert.ok(bridge);
  assert.ok(Object.isFrozen(bridge));
  await bridge.startWindowDrag({x: 100, y: 200});
  assert.deepEqual(calls, [{channel: "note:window-drag-start", payload: {x:100, y:200}}]);
  let seen = "";
  const unsubscribe = bridge.onAuxiliaryOpened(value => { seen = value.role; });
  listeners.get("note:auxiliary-opened")!({}, {role:"settings"});
  assert.equal(seen, "settings");
  unsubscribe();
  assert.equal(listeners.size, 0);
});
