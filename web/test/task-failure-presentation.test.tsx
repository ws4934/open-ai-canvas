import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { renderToStaticMarkup } from "react-dom/server";
import { Window } from "happy-dom";

import { ErrorContent } from "@/components/canvas/canvas-node-status-content";
import { TaskListItem } from "@/components/canvas/canvas-workspace-task-list-item";
import { GenerationBatchDetails } from "@/components/canvas/canvas-script-node-parts";
import { canvasThemes } from "@/lib/canvas-theme";
import { nodeHoverToolbarTools } from "@/lib/canvas/tool-registry/definitions/node-hover-tools";
import type { ToolContext } from "@/lib/canvas/tool-registry";
import { CONTENT_MODERATION_MESSAGE } from "@/lib/generation-error";
import { TaskGridCard } from "@/pages/tasks/task-grid-card";
import { TaskGroupHeader } from "@/pages/tasks/task-group-header";
import { TaskListRow } from "@/pages/tasks/task-list-row";
import { taskAttentionReason } from "@/pages/tasks/task-shared";
import type { GenerationTask } from "@/services/api/task-center";
import type { AiConfig } from "@/stores/use-config-store";
import { CanvasNodeType, type CanvasGenerationBatch, type CanvasNodeData } from "@/types/canvas";

const failedTask: GenerationTask = {
    id: "failure-fixture",
    type: "canvas_image",
    status: "failed",
    stage: "任务失败",
    prompt: "test prompt",
    attempts: 1,
    createdAt: "2026-10-28T12:00:00Z",
    updatedAt: "2026-10-28T12:01:00Z",
    error: "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。",
};

describe("任务失败展示", () => {
    test("真实画布任务列表项直接展示各类任务的失败原因", () => {
        for (const type of ["canvas_image", "canvas_video", "canvas_audio", "canvas_text", "timeline_render", "timeline_transcription"]) {
            const html = renderToStaticMarkup(<TaskListItem task={{ ...failedTask, type }} />);
            expect(html).toContain(CONTENT_MODERATION_MESSAGE);
            expect(html).not.toContain("；上游：");
        }
    });

    test("创作历史显示原因，保存失败保留交付阶段和具体错误", () => {
        expect(taskAttentionReason(failedTask)).toContain("内容审核未通过");
        const saving = taskAttentionReason({ ...failedTask, mediaStage: "register", error: "素材登记失败，请联系管理员。" });
        expect(saving).toContain("登记素材失败");
        expect(saving).toContain("素材登记失败，请联系管理员");
        expect(taskAttentionReason({ ...failedTask, error: undefined })).toContain("未返回具体原因");
    });

    test("图片与视频工作台主结果详情入口查询完整任务", () => {
        const source = readFileSync(new URL("../src/pages/workbench/image-workbench-page.tsx", import.meta.url), "utf8");
        expect(source.match(/onClick=\{\(\) => void openDetail\(resultTask\)\}/g)).toHaveLength(2);
        expect(source).not.toContain("onClick={() => setDetailTask(resultTask)}");
        expect(source).toContain("generationTaskFailureReason(resultTask)");
        expect(source).toContain("generationTaskFailureReason(detailTask)");
        expect(source).toContain("generationTaskFailureReason(task)");
        expect(source).toContain("recoveringMedia ? await recoverGenerationTaskMedia(task.id) : await retryGenerationTask(task.id)");
    });

    test("审核失败在创作历史网格、列表和分组中仍显示可用的重试", () => {
        const task = { ...failedTask, errorCode: "sensitive_words_detected" };
        const action = () => {};
        const variants = [
            <TaskGridCard task={task} actingId="" onOpen={action} onRetry={action} />,
            <TaskListRow task={task} canvasById={new Map()} projectNameById={new Map()} effectiveConfig={{} as AiConfig} creditsEnabled={false} actingId="" onOpen={action} onRetry={action} onPreview={action} />,
        ];
        const window = new Window();
        for (const variant of variants) {
            window.document.body.innerHTML = renderToStaticMarkup(variant);
            const retry = window.document.querySelector('button[aria-label="重试任务"]');
            expect(retry).not.toBeNull();
            expect(retry?.hasAttribute("disabled")).toBe(false);
            expect(window.document.body.textContent).toContain(CONTENT_MODERATION_MESSAGE);
        }
        const grouped = renderToStaticMarkup(<TaskGroupHeader group={{ key: "group", title: "画布", projectName: "", tasks: [task] }} onRetryFailed={action} />);
        expect(grouped).toContain("1 失败");
        expect(grouped).toContain("重试失败任务");
        expect(taskAttentionReason(task)).not.toContain("新建任务");
    });

    test("审核失败画布节点在明暗主题均保留重新生成入口", () => {
        const node: CanvasNodeData = { id: "node", type: CanvasNodeType.Image, title: "图片", position: { x: 0, y: 0 }, width: 320, metadata: { status: "error", errorDetails: failedTask.error, generationErrorCode: "sensitive_words_detected" } };
        for (const theme of Object.values(canvasThemes)) {
            const html = renderToStaticMarkup(<ErrorContent node={node} theme={theme} onRetry={() => {}} />);
            expect(html).toContain(CONTENT_MODERATION_MESSAGE);
            expect(html).toMatch(/<button[^>]*>.*?重新生成<\/button>/s);
        }
        const calls: CanvasNodeData[] = [];
        const context = { node, nodeMetadata: node.metadata, handlers: { onNodeRetry: (value: CanvasNodeData) => calls.push(value) } } as ToolContext;
        const retry = nodeHoverToolbarTools.find((tool) => tool.id === "retry")!;
        expect(retry.applicable?.(context)).toBe(true);
        retry.run(context);
        expect(calls).toEqual([node]);
        expect(retry.applicable?.({ ...context, nodeMetadata: { ...node.metadata, status: "loading" } })).toBe(false);
        expect(retry.applicable?.({ ...context, nodeMetadata: { ...node.metadata, fileUpload: "error" } })).toBe(false);
    });

    test("分镜批次审核失败项保留重试按钮，不再强制修改提示词", () => {
        const batch = { id: "batch", mode: "storyboard_image", items: [{ id: "item", rowId: "row", nodeId: "node", status: "failed", errorDetails: failedTask.error, retryCount: 0 }] } as CanvasGenerationBatch;
        const html = renderToStaticMarkup(<GenerationBatchDetails batch={batch} rows={[]} onRetryItem={() => {}} />);
        expect(html).toContain("审核未通过");
        expect(html).toContain('aria-label="重试镜头 "');
        expect(html).not.toContain("需修改提示词");
        for (const file of ["use-canvas-generation-retry.ts", "use-canvas-generation-batches.ts"]) {
            const source = readFileSync(new URL(`../src/pages/canvas/${file}`, import.meta.url), "utf8");
            expect(source).not.toContain("unchangedModeratedPrompt");
            expect(source).not.toContain("未通过内容审核");
        }
    });
});
