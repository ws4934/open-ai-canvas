import type { GenerationTask } from "@/services/api/task-center";

export type GenerationTaskMediaSource = Pick<GenerationTask, "type" | "previewKind" | "previewUrl" | "previewImages" | "resultJson">;
export type GenerationTaskMedia = { kind: "image" | "video" | "audio"; url: string };

function taskResult(task: GenerationTaskMediaSource): Record<string, unknown> | null {
    if (!task.resultJson) return null;
    try {
        const result: unknown = JSON.parse(task.resultJson);
        return result && typeof result === "object" && !Array.isArray(result) ? (result as Record<string, unknown>) : null;
    } catch {
        return null;
    }
}

function mediaURL(value: unknown): string {
    if (typeof value === "string") return value.trim();
    if (!value || typeof value !== "object" || Array.isArray(value)) return "";
    const item = value as Record<string, unknown>;
    return typeof item.dataUrl === "string" && item.dataUrl.trim() ? item.dataUrl.trim() : typeof item.url === "string" ? item.url.trim() : "";
}

/** 按产物读取结果，不按地址后缀猜测，也不把同一产物的 url/dataUrl 算成两张。 */
export function generationTaskMedia(task: GenerationTaskMediaSource): GenerationTaskMedia[] {
    const result = taskResult(task);
    if (task.previewKind === "video" || task.type.includes("video") || result?.mode === "video") {
        const url = mediaURL(result?.video) || task.previewUrl || "";
        return url ? [{ kind: "video", url }] : [];
    }
    if (task.type.includes("audio") || result?.mode === "audio") {
        const audios = Array.isArray(result?.audios) && result.audios.length ? result.audios : [result?.audio];
        return audios
            .map(mediaURL)
            .filter(Boolean)
            .map((url) => ({ kind: "audio", url }));
    }
    if (Array.isArray(result?.images)) {
        const images = result.images.map(mediaURL).filter(Boolean);
        if (images.length) return images.map((url) => ({ kind: "image", url }));
    }
    if (task.previewImages?.length) return task.previewImages.map((url) => ({ kind: "image", url }));
    return task.previewUrl ? [{ kind: "image", url: task.previewUrl }] : [];
}

export function generationTaskImageUrls(task: GenerationTaskMediaSource): string[] {
    return generationTaskMedia(task)
        .filter((media) => media.kind === "image")
        .map((media) => media.url);
}

export function generationTaskPreviewUrl(task: GenerationTaskMediaSource): string {
    return generationTaskMedia(task)[0]?.url || "";
}
