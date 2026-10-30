import type { DramaSegment } from "@/services/api/drama";

export function secondsLabel(ms: number) {
    const seconds = Math.round(ms / 100) / 10;
    return `${Number.isInteger(seconds) ? seconds : seconds.toFixed(1)}s`;
}

export function segmentDurationLabel(segment: DramaSegment) {
    if (segment.videoResourceId && segment.videoSeconds) return `${segment.videoSeconds}s`;
    if (segment.request.seconds) return `${segment.request.seconds}s`;
    return `分镜 ${secondsLabel(segment.durationMs)}`;
}

export function segmentTimingLabel(segment: DramaSegment) {
    const planned = `分镜 ${secondsLabel(segment.durationMs)}`;
    if (segment.videoResourceId && segment.videoSeconds) return `${planned} · 已生成 ${segment.videoSeconds}s`;
    if (segment.request.seconds) return `${planned} · 生成 ${segment.request.seconds}s`;
    return planned;
}

export function segmentPlaybackStarts(segments: DramaSegment[]) {
    let cursor = 0;
    return segments.map((segment) => {
        const start = cursor;
        cursor += segment.videoResourceId && segment.videoSeconds ? segment.videoSeconds : segment.request.seconds || segment.durationMs / 1000;
        return start;
    });
}
