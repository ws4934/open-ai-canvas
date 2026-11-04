import { expect, test } from "bun:test";
import { resolve } from "node:path";

// React DOM 的环境检测需要在独立进程中先安装 DOM，避免受其他 SSR 用例影响。
test("真实连接控制器的悬停、释放、取消及重叠节点回归", async () => {
    const child = Bun.spawn([process.execPath, "test", "./test/fixtures/canvas-connection-controller.harness.tsx"], { cwd: resolve(import.meta.dir, ".."), timeout: 20000, stdout: "pipe", stderr: "pipe" });
    const [exitCode, stdout, stderr] = await Promise.all([child.exited, new Response(child.stdout).text(), new Response(child.stderr).text()]);
    expect(exitCode, stdout + stderr).toBe(0);
}, 25000);
