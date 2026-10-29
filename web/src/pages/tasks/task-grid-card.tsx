import { Button } from "antd";
import { IconButton } from "@/components/ui/base/buttons";
import { Tooltip } from "@/components/ui/base/tooltip";
import { Eye, FileText, Image as ImageIcon, RotateCcw, Trash2, Video } from "lucide-react";
import { Checkbox } from "@/components/ui/base/checkbox/checkbox";

import { MediaPreview } from "@/components/media-preview";
import { generationTaskShowsProgress, generationTaskStageLabel, generationTaskStatusLabel } from "@/lib/generation-task-display";
import type { GenerationTask } from "@/services/api/task-center";
import { isTaskFailed, statusDotClassName, taskAttentionReason, TaskDate } from "./task-shared";
import { TaskVideoThumbnail } from "./task-video-thumbnail";
import { progressFillStyle } from "@/lib/progress-fill";

export function TaskGridCard({ task, actingId, onOpen, onRetry, onClear, onSelect, selected, clearing }: { task: GenerationTask; actingId: string; onOpen: () => void; onRetry: () => void; onClear?: () => void; onSelect?: () => void; selected?: boolean; clearing?: boolean }) {
    const isActive = task.status === "queued" || task.status === "running";
    const isFailed = isTaskFailed(task);
    const isVideo = task.previewKind === "video";
    const thumbnailUrl = isVideo ? task.previewPosterUrl : task.previewUrl;
    const fallbackVideo = task.type.includes("video");
    const stageLabel = generationTaskStageLabel(task);
    const showsProgress = generationTaskShowsProgress(task);
    const Icon = fallbackVideo ? Video : task.type.includes("image") ? ImageIcon : FileText;
    return (
        <article className={`product-collection-card task-grid-card${isFailed ? " is-attention" : ""}`}>
            <div className="task-grid-thumb">
                {onSelect ? <Checkbox className="absolute left-2 top-2 z-10 rounded bg-background p-2" checked={selected} disabled={clearing || actingId === task.id} onChange={onSelect} aria-label={`选择任务：${task.prompt || "未命名任务"}`} /> : null}
                {thumbnailUrl ? (
                    <MediaPreview src={thumbnailUrl} kind="image" loading="lazy" className="h-full w-full object-cover" />
                ) : isVideo && task.previewUrl ? (
                    <TaskVideoThumbnail src={task.previewUrl} />
                ) : (
                    <Icon />
                )}
                <div className="task-grid-overlay">
                    <Tooltip title="查看详情">
                        <IconButton size="sm" variant="ghost" icon={Eye} aria-label="查看详情" onClick={onOpen} />
                    </Tooltip>
                    {isFailed ? (
                        <Tooltip title={task.canRecoverMedia ? "重试保存，不重新生成" : "重试任务"}>
                            <Button
                                type="text"
                                size="small"
                                icon={<RotateCcw className="size-3.5" />}
                                aria-label={task.canRecoverMedia ? "重试保存" : "重试任务"}
                                loading={actingId === task.id}
                                onClick={onRetry}
                            />
                        </Tooltip>
                    ) : null}
                    {onClear ? <Tooltip title="清理任务记录"><IconButton size="sm" variant="ghost" icon={Trash2} aria-label="清理任务记录" disabled={clearing || actingId === task.id} onClick={onClear} /></Tooltip> : null}
                </div>
            </div>
            <div className="task-grid-body">
                <button type="button" className="task-grid-title" title={task.prompt} onClick={onOpen}>
                    {task.prompt || "未命名任务"}
                </button>
                <div className="task-grid-meta">
                    <span className={`task-grid-status ${isFailed ? "is-failed" : isActive ? "is-active" : task.status === "succeeded" ? "is-success" : ""}`}>
                        <i className={statusDotClassName(task.status)} />
                        {generationTaskStatusLabel(task)}
                    </span>
                    <span className="task-grid-date">
                        <TaskDate value={task.createdAt} />
                    </span>
                </div>
                {isActive && !showsProgress ? <p>{stageLabel}</p> : isActive ? (
                    <div className="task-grid-progress" role="progressbar" aria-label={stageLabel} aria-valuemin={0} aria-valuemax={100} aria-valuenow={task.progress || 0}>
                        <span>{stageLabel}</span>
                        <strong>{task.progress || 0}%</strong>
                        <i><b style={progressFillStyle(task.progress || 0)} /></i>
                    </div>
                ) : null}
                {isFailed ? <p className="task-grid-error" title={task.error || undefined}>{taskAttentionReason(task)}</p> : null}
            </div>
        </article>
    );
}
