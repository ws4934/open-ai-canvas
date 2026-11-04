import type { SystemUpdateStatus, UpdatePhase } from "@/services/api/system-update";

export const updatePhaseLabels: Record<UpdatePhase, string> = {
    idle: "等待更新",
    downloading: "下载程序",
    validating: "校验程序",
    draining: "等待任务完成",
    switching: "安装新版",
    restarting: "正在重启",
    succeeded: "更新成功",
    rolled_back: "已回退",
    failed: "更新失败",
    manual_intervention: "需要恢复程序",
};

export function isUpdateActive(phase?: UpdatePhase) {
    return phase === "downloading" || phase === "validating" || phase === "draining" || phase === "switching" || phase === "restarting";
}

export function needsUpdatedPage(status: SystemUpdateStatus, pageVersion: string) {
    return Boolean(status.operation.id && (status.operation.phase === "succeeded" || status.operation.phase === "rolled_back") && status.currentVersion.replace(/^v/, "") !== pageVersion.replace(/^v/, ""));
}
