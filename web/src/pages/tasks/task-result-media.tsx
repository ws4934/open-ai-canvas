import { Typography } from "antd";

import { MediaPreview } from "@/components/media-preview";
import { generationTaskMedia, type GenerationTaskMediaSource } from "@/lib/generation-task-media";

export function TaskResultMedia({ task }: { task: GenerationTaskMediaSource }) {
    const media = generationTaskMedia(task);
    if (!media.length) return null;
    return (
        <div>
            <Typography.Text strong>生成结果{media.length > 1 ? ` · ${media.length} 项` : ""}</Typography.Text>
            <div className="mt-2 grid max-h-[360px] grid-cols-2 gap-2 overflow-auto rounded-lg bg-stone-950 p-2 md:grid-cols-3">
                {media.map(({ url, kind }, index) =>
                    kind === "audio" ? (
                        <audio key={`${index}:${url}`} src={url} controls preload="metadata" aria-label={`生成结果 ${index + 1}`} className="w-full" />
                    ) : (
                        <MediaPreview
                            key={`${index}:${url}`}
                            src={url}
                            kind={kind}
                            alt={`生成结果 ${index + 1}`}
                            controls={kind === "video"}
                            loading="lazy"
                            className={kind === "video" ? "task-result-media is-video" : "task-result-media"}
                            fallbackClassName={kind === "video" ? "task-result-media is-video" : "task-result-media"}
                        />
                    ),
                )}
            </div>
        </div>
    );
}
