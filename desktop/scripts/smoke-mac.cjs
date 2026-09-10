const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const assert = require("node:assert/strict");

async function main() {
  assert.equal(process.platform, "darwin");
  const folder = process.arch === "arm64" ? "mac-arm64" : "mac";
  const bundle = path.resolve(__dirname, "..", "release", folder, "Note.app");
  // __dirname is desktop/scripts, so the output is one level above scripts.
  const backend = path.join(bundle, "Contents/Resources/backend/note-api");
  fs.accessSync(backend, fs.constants.X_OK);
  const log = path.join(os.homedir(), "Library/Logs/note-desktop/backend.log");
  const child = spawn(path.join(bundle, "Contents/MacOS/Note"), [], { stdio: "inherit" });
  let error;
  child.on("error", value => { error = value; });
  try {
    let url;
    for (let attempt = 0; attempt < 60; attempt++) {
      if (error) throw error;
      assert.equal(child.exitCode, null, "Electron exited before startup");
      if (fs.existsSync(log)) {
        url = fs.readFileSync(log, "utf8").match(/NOTE_SERVER_URL=(http:\/\/[^\s]+)/)?.[1];
      }
      if (url) break;
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
    assert.ok(url, "Packaged Electron did not start the bundled backend");
    for (const route of ["/api/ping", "/", "/api/calendar?from=2026-09-10&to=2026-09-11"]) {
      const response = await fetch(url + route);
      assert.equal(response.status, 200, route);
      if (route === "/") assert.match(await response.text(), /<html/i);
    }
    console.log(`Packaged ${process.arch} Electron, backend, SQLite and web smoke checks passed`);
  } finally {
    child.kill("SIGTERM");
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
