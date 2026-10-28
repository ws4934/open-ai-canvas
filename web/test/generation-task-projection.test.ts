import { describe, expect, test } from "bun:test";

import type { GenerationTask } from "@/services/api/task-center";
import { createGenerationTaskMaterializer, type GenerationTaskEffectResult } from "@/services/generation-task-materializer";
import { projectGenerationTaskResult } from "@/services/project-asset-sync";

const summary: GenerationTask = {
    id: "multi-result-summary",
    type: "canvas_image",
    status: "succeeded",
    prompt: "测试",
    attempts: 1,
    createdAt: "2026-10-28T12:00:00Z",
    updatedAt: "2026-10-28T12:01:00Z",
    previewUrl: "/api/resources/a/file",
    previewKind: "image",
    previewImages: ["/api/resources/a/file", "/api/resources/b/file"],
};

describe("任务素材完整输出投影", () => {
    test("摘要中的全部图片都生成输出，并保留可供后续消费者解析的结果", () => {
        const projected = projectGenerationTaskResult(summary);
        expect(projected.outputs?.map((output) => output.outputIndex)).toEqual([0, 1]);
        expect(JSON.parse(projected.resultJson!).images.map((image: { dataUrl: string }) => image.dataUrl)).toEqual(summary.previewImages);
        expect(projected.resultState).toBe("PENDING_MATERIALIZATION");
        expect(projectGenerationTaskResult({ ...summary, previewUrl: undefined }).outputs).toHaveLength(2);
        expect(summary.resultJson).toBeUndefined();
    });

    test("预览不全时不删除已知输出，也不丢失已绑定的素材与状态", () => {
        const outputs = [
            { outputIndex: 0, mediaType: "image" as const, materializedAssetId: "asset-a", providerArtifactRef: "resource:a" },
            { outputIndex: 1, mediaType: "image" as const, materializedAssetId: "asset-b", providerArtifactRef: "resource:b" },
        ];
        const projected = projectGenerationTaskResult({ ...summary, previewImages: undefined, outputs, resultState: "READY" });
        expect(projected.outputs).toEqual(outputs);
        expect(projected.resultState).toBe("READY");
        expect(projectGenerationTaskResult({ ...summary, outputs }).outputs).toEqual(outputs);
    });

    test("完整详情优先于摘要，保留全部产物尺寸与素材标识", () => {
        const images = Array.from({ length: 32 }, (_, index) => ({ dataUrl: `/api/resources/full-${index}/file`, storageKey: `resource:full-${index}`, width: 1024, height: 768 }));
        const projected = projectGenerationTaskResult({ ...summary, resultJson: JSON.stringify({ mode: "image", images }) });
        expect(projected.outputs).toHaveLength(32);
        expect(projected.outputs?.[31]).toEqual({ outputIndex: 31, mediaType: "image", providerArtifactRef: "resource:full-31" });
        expect(JSON.parse(projected.resultJson!).images).toEqual(images);
    });

    test("摘要恢复经过真实物化器时保存每个输出，重复恢复不再保存或生成", async () => {
        const completed = new Map<string, GenerationTaskEffectResult>();
        const saved: string[] = [];
        const materializer = createGenerationTaskMaterializer({
            effects: {
                async claim(key) {
                    const result = completed.get(key);
                    return result ? { status: "completed", result } : { status: "claimed", fence: 1 };
                },
                async renew() {
                    return { fence: 1 };
                },
                async complete(key, _taskId, result) {
                    completed.set(key, result);
                },
                async release() {},
            },
            async materializeOutput({ task, output }) {
                saved.push(JSON.parse(task.resultJson!).images[output.outputIndex].dataUrl);
                return { materializedAssetId: `asset-${output.outputIndex}` };
            },
        });
        const recovered = await materializer.materialize(projectGenerationTaskResult(summary));
        expect(saved).toEqual(summary.previewImages!);
        expect(recovered.outputs?.map((output) => output.materializedAssetId)).toEqual(["asset-0", "asset-1"]);
        expect(recovered.resultState).toBe("READY");
        const replayed = await materializer.materialize(projectGenerationTaskResult(summary));
        expect(replayed.outputs).toEqual(recovered.outputs);
        expect(saved).toHaveLength(2);
    });

    test("显式结果与单图、视频仍使用各自的输出合同", () => {
        expect(projectGenerationTaskResult({ ...summary, previewImages: undefined }).outputs).toHaveLength(1);
        const video = projectGenerationTaskResult({ ...summary, type: "canvas_video", previewKind: "video", previewUrl: "/api/resources/video/file" });
        expect(video.outputs).toEqual([{ outputIndex: 0, mediaType: "video" }]);
        expect(JSON.parse(video.resultJson!).video.dataUrl).toBe("/api/resources/video/file");
        const explicit = projectGenerationTaskResult(summary, { mode: "image", images: [{ dataUrl: "/new.png", storageKey: "resource:new" }] });
        expect(explicit.outputs).toEqual([{ outputIndex: 0, mediaType: "image", providerArtifactRef: "resource:new" }]);
        const replaced = projectGenerationTaskResult(
            { ...summary, outputs: [{ outputIndex: 0, mediaType: "image", providerArtifactRef: "resource:old", materializedAssetId: "old-asset" }] },
            { images: [{ dataUrl: "/new.png", storageKey: "resource:new" }] },
        );
        expect(replaced.outputs?.[0].materializedAssetId).toBeUndefined();
    });
});
