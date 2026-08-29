const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const projectRoot = path.resolve(__dirname, "..", "..");
const desktopRoot = path.join(projectRoot, "desktop");
const webRoot = path.join(projectRoot, "web");
const resourcesRoot = path.join(desktopRoot, "resources");
const npmCLI = process.env.npm_execpath;
const backendName = process.platform === "win32" ? "note-api.exe" : "note-api";

fs.mkdirSync(resourcesRoot, { recursive: true });

if (!npmCLI) throw new Error("npm_execpath is unavailable; run this script through npm");

run(process.execPath, [npmCLI, "run", "build"], webRoot);
run(
  "go",
  ["build", "-trimpath", "-ldflags=-s -w", "-o", path.join(resourcesRoot, backendName), "./cmd/api"],
  projectRoot,
);

function run(command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd,
    stdio: "inherit",
    shell: false,
  });

  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
