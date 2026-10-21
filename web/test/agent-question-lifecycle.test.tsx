import { expect, test } from "bun:test";
import { resolve } from "node:path";

// React DOM caches input-event support at its first import. Keep the real DOM
// lifecycle tests independent of SSR suites that import it before happy-dom.
test("Agent 提问真实组件的生命周期、空默认值与中文多选输入", () => {
    const result = Bun.spawnSync([process.execPath, "test", "./test/fixtures/agent-question-lifecycle.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 15000 });
    const output = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
    expect(output).toContain("8 pass");
    expect(output).toContain("0 fail");
    expect(result.exitCode).toBe(0);
}, 20000);
