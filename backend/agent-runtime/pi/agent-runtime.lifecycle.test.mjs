import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtemp, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const runtime = fileURLToPath(new URL("./agent-runtime.mjs", import.meta.url));

test("runtime refuses missing or symlinked parent-owned directories", async t => {
  const directory = await mkdtemp(join(tmpdir(), "agent-directory-contract-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const linked = join(directory, "linked");
  await symlink(directory, linked);
  for (const runDir of [undefined, linked]) {
    const child = spawn(process.execPath, [runtime], {
      env: { PATH: process.env.PATH, NODE_ENV: "production", PI_OFFLINE: "1", HOME: directory, ...(runDir ? { YINGCE_AGENT_RUN_DIR: runDir } : {}) },
      stdio: ["pipe", "pipe", "pipe"],
    });
    let output = "";
    child.stdout.on("data", chunk => { output += chunk; });
    child.stderr.resume();
    const exited = new Promise(resolve => child.once("close", resolve));
    const timeout = setTimeout(() => child.kill("SIGKILL"), 10000);
    child.stdin.end(JSON.stringify({ bridgeURL: "http://127.0.0.1:1", bridgeToken: "test", model: { id: "test" }, prompt: "test", runDir: directory }));
    const code = await exited;
    clearTimeout(timeout);
    assert.equal(code, 1);
    assert.match(output, /temporary directory/);
  }
});
