import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { TaskHistoryToolbar } from "@/components/tasks/task-history-toolbar";
import type { TaskHistoryCleanup } from "@/hooks/use-task-history-cleanup";
import { TaskGridCard } from "@/pages/tasks/task-grid-card";
import { apiClient } from "@/services/api/request";
import { canClearTaskHistory, clearGenerationTaskHistory, type GenerationTask } from "@/services/api/task-center";

test("清理只有已结束任务可选，批量入口明确限于本页", () => {
    for (const status of ["queued", "running"] as const) expect(canClearTaskHistory({ status })).toBe(false);
    for (const status of ["succeeded", "failed", "cancelled"] as const) expect(canClearTaskHistory({ status })).toBe(true);
    const cleanup = { managing: true, clearing: false, selectedIds: ["a", "b"], eligibleCount: 3, selectAll() {}, toggleManaging() {}, clearSelected() {} } as unknown as TaskHistoryCleanup;
    const toolbar = renderToStaticMarkup(<TaskHistoryToolbar cleanup={cleanup} />);
    expect(toolbar).toContain("全选本页");
    expect(toolbar).toContain("清理所选 (2)");
    expect(toolbar).toContain("取消管理");
    const task = { id: "a", status: "failed", prompt: "测试", type: "canvas_image", createdAt: "2026-01-01T00:00:00Z" } as GenerationTask;
    const card = renderToStaticMarkup(<TaskGridCard task={task} actingId="" onOpen={() => {}} onRetry={() => {}} onClear={() => {}} onSelect={() => {}} selected />);
    expect(card).toContain('aria-label="清理任务记录"');
    expect(card).toContain('aria-label="选择任务：测试"');
    expect(card).toContain('aria-label="重试任务"');
});

test("单条与批量清理使用同一写接口，部分跳过保留机器原因，写失败向上传递", async () => {
    const original = apiClient.request;
    const result = { clearedIds: ["a"], skipped: [{ id: "b", reason: "active" }] };
    const calls: unknown[] = [];
    apiClient.request = (async (config: { url: string; method: string; data: unknown }) => {
        calls.push([config.url, config.method, config.data]);
        if (calls.length > 1) throw new Error("connection interrupted");
        return { status: 200, data: { code: 0, data: result, msg: "" } };
    }) as typeof apiClient.request;
    try {
        expect(await clearGenerationTaskHistory(["a", "b"])).toEqual(result);
        expect(calls[0]).toEqual(["/tasks/clear-history", "post", { ids: ["a", "b"] }]);
        await expect(clearGenerationTaskHistory(["a"])).rejects.toThrow("connection interrupted");
    } finally {
        apiClient.request = original;
    }
});
