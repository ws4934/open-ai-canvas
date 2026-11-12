import { dataUrlToFile } from "@/lib/image-utils";
import { modelCapabilityConfigFor, videoResolutionRequest } from "@/lib/model-capabilities";
import { imageToDataUrl } from "@/services/image-storage";
import { modelOptionName } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";

import { normalizeVideoSeconds, normalizeVideoSize } from "./video-validation";
import type { RequestOptions, ResolvedAiConfig, ApiVideoResponse, VideoGenerationTask, VideoGenerationTaskState } from "./video-contracts";
import type { VideoProviderDeps } from "./video-provider-deps";

export async function createOpenAIVideoTask(deps: VideoProviderDeps, config: ResolvedAiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const modelName = modelOptionName(model);
    if (config.interfaceType === "xai-video" || modelName.toLowerCase().includes("grok")) {
        const selectedReferences = config.interfaceType === "xai-video" ? references : references.slice(0, 7);
        const images = await Promise.all(selectedReferences.map((image) => imageToDataUrl(image)));
        const seconds = normalizeVideoSeconds(config.videoSeconds);
        const referenceMode = options?.videoEditOperation === "reference_to_video";
        const imagePayload: Record<string, unknown> = {};
        if (config.interfaceType === "xai-video") {
            const first = selectedReferences.flatMap((image, index) => (!referenceMode && (image.id === options?.videoStartFrameNodeId || image.role === "first_frame") ? [index] : []));
            const last = selectedReferences.flatMap((image, index) => (!referenceMode && (image.id === options?.videoEndFrameNodeId || image.role === "last_frame") ? [index] : []));
            if (first.length > 1 || last.length > 1 || first.some((index) => last.includes(index))) throw new Error("xAI 视频协议每种帧最多一张，请分别指定起始帧和结束帧");
            if (first.length) imagePayload.image = { url: images[first[0]] };
            if (last.length) imagePayload.last_frame = { url: images[last[0]] };
            const refs = images.filter((_, index) => !first.includes(index) && !last.includes(index));
            if (referenceMode || first.length || last.length || selectedReferences.some((image) => image.role === "reference")) {
                if (refs.length) imagePayload.reference_images = refs.map((url) => ({ url }));
            } else if (images.length) imagePayload.image = { url: images[0] };
        } else if (images.length) Object.assign(imagePayload, { image: images[0], images });
        const payload = {
            model: modelName,
            prompt,
            duration: Number.parseInt(seconds, 10) || 6,
            ...(config.interfaceType === "xai-video" ? { aspect_ratio: config.size || "16:9", resolution: config.vquality || "720p" } : { seconds, ...(normalizeVideoSize(config.size) ? { size: normalizeVideoSize(config.size) } : {}) }),
            ...imagePayload,
        };
        try {
            const createPath = config.interfaceType === "xai-video" ? "/videos/generations" : "/videos";
            const created = deps.response.unwrapVideoResponse(await deps.transport.post<ApiVideoResponse>(deps.transport.apiUrl(createPath), payload, options));
            const id = deps.response.videoTaskId(created);
            if (!id) throw new Error("视频接口没有返回任务 ID");
            return { id, provider: "openai", model };
        } catch (error) {
            throw new Error(deps.response.readAxiosError(error, "视频任务创建失败"));
        }
    }
    const body = new FormData();
    body.append("model", modelName);
    body.append("prompt", prompt);
    body.append("seconds", normalizeVideoSeconds(config.videoSeconds));
    if (normalizeVideoSize(config.size)) body.append("size", normalizeVideoSize(config.size)!);
    const resolution = videoResolutionRequest(modelCapabilityConfigFor(config, model).video!, config.vquality);
    if (resolution) body.append("resolution_name", resolution);
    body.append("preset", "normal");
    const files = await Promise.all(references.slice(0, 7).map(async (image) => dataUrlToFile({ ...image, dataUrl: await imageToDataUrl(image) })));
    files.forEach((file) => body.append("input_reference[]", file));
    try {
        const created = deps.response.unwrapVideoResponse(await deps.transport.postForm<ApiVideoResponse>(deps.transport.apiUrl("/videos"), body, options));
        if (!created.id) throw new Error("视频接口没有返回任务 ID");
        return { id: created.id, provider: "openai", model };
    } catch (error) {
        throw new Error(deps.response.readAxiosError(error, "视频任务创建失败"));
    }
}

export async function pollOpenAIVideoTask(deps: VideoProviderDeps, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const video = deps.response.unwrapVideoResponse(await deps.transport.get<ApiVideoResponse>(deps.transport.apiUrl(`/videos/${task.id}`), options));
        if (video.status === "completed" || video.status === "succeeded" || video.status === "success" || video.status === "done") {
            const resultUrl = video.video?.url || video.video_url || video.result_url;
            if (resultUrl) return { status: "completed", result: await deps.response.videoResultFromUrl(resultUrl, options) };
            const content = await deps.transport.getBlob(deps.transport.apiUrl(`/videos/${task.id}/content`), options);
            await deps.response.assertVideoBlob(content);
            return { status: "completed", result: { blob: content } };
        }
        if (video.status === "failed" || video.status === "cancelled") return { status: "failed", error: video.error?.message || "视频生成失败" };
        return { status: "pending" };
    } catch (error) {
        throw new Error(deps.response.readAxiosError(error, "视频任务查询失败"));
    }
}
