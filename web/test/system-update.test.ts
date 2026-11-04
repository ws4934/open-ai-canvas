import { expect, test } from "bun:test";
import type { SystemUpdateStatus } from "../src/services/api/system-update";
import { isUpdateActive, needsUpdatedPage, updatePhaseLabels } from "../src/pages/admin/settings/system-update-state";

const status: SystemUpdateStatus = {
    supported: true,
    repository: "ws4934/open-ai-canvas",
    currentVersion: "v1.6.9.1",
    updateAvailable: false,
    canRollback: true,
    rollbackVersion: "v1.6.9",
    operation: { id: "update-1", phase: "succeeded", automaticRollback: false, logs: [] },
};

test("download, validation, drain, replacement and restart keep polling", () => {
    for (const phase of ["downloading", "validating", "draining", "switching", "restarting"] as const) expect(isUpdateActive(phase)).toBe(true);
    for (const phase of ["idle", "failed", "manual_intervention", "succeeded", "rolled_back"] as const) expect(isUpdateActive(phase)).toBe(false);
    for (const label of Object.values(updatePhaseLabels)) expect(label.length).toBeGreaterThan(0);
});

test("refresh requires a confirmed terminal operation and a changed running version", () => {
    expect(needsUpdatedPage(status, "v1.6.9")).toBe(true);
    expect(needsUpdatedPage(status, "1.6.9.1")).toBe(false);
    expect(needsUpdatedPage({ ...status, operation: { ...status.operation, phase: "restarting" } }, "v1.6.9")).toBe(false);
    expect(needsUpdatedPage({ ...status, operation: { ...status.operation, phase: "failed" } }, "v1.6.9")).toBe(false);
    expect(needsUpdatedPage({ ...status, operation: { ...status.operation, phase: "rolled_back" } }, "v1.7.0")).toBe(true);
    expect(needsUpdatedPage({ ...status, operation: { ...status.operation, id: undefined } }, "v1.6.9")).toBe(false);
});
