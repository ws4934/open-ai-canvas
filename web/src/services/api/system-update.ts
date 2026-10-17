import { http } from "./request";

export type UpdatePhase = "idle" | "downloading" | "validating" | "draining" | "switching" | "restarting" | "succeeded" | "rolled_back" | "failed" | "manual_intervention";

export type SystemUpdateRelease = {
    version: string;
    name: string;
    body: string;
    url: string;
    publishedAt: string;
};

export type SystemUpdateOperation = {
    id?: string;
    kind?: "update" | "rollback";
    phase: UpdatePhase;
    fromVersion?: string;
    targetVersion?: string;
    startedAt?: string;
    finishedAt?: string;
    error?: string;
    automaticRollback: boolean;
    logs: { at: string; phase: UpdatePhase; message: string }[];
};

export type SystemUpdateStatus = {
    supported: boolean;
    unsupportedReason?: string;
    repository: string;
    currentVersion: string;
    latestRelease?: SystemUpdateRelease;
    updateAvailable: boolean;
    checkedAt?: string;
    canRollback: boolean;
    rollbackVersion?: string;
    operation: SystemUpdateOperation;
};

export function getSystemUpdateStatus(signal?: AbortSignal) {
    return http.get<SystemUpdateStatus>("/admin/system-update", { signal, timeout: 10000 });
}

export function checkSystemUpdate(force = true, signal?: AbortSignal) {
    return http.post<SystemUpdateStatus>("/admin/system-update/check", { force }, { signal, timeout: 45000 });
}

export function startSystemUpdate(targetVersion: string) {
    return http.post<SystemUpdateStatus>("/admin/system-update/start", { targetVersion });
}

export function rollbackSystemUpdate(reason: string) {
    return http.post<SystemUpdateStatus>("/admin/system-update/rollback", { reason });
}
