import { useEffect, useRef, useState } from "react";
import { App } from "antd";
import { useQueryClient } from "@tanstack/react-query";

import { canClearTaskHistory, clearGenerationTaskHistory, type GenerationTask } from "@/services/api/task-center";
import { useUserStore } from "@/stores/use-user-store";

export function useTaskHistoryCleanup(tasks: GenerationTask[], scope: string, onCleared: (ids: string[]) => void) {
    const { modal, message } = App.useApp();
    const userId = useUserStore((state) => state.user?.id || "");
    const queryClient = useQueryClient();
    const selectionScope = `${userId}:${scope}`;
    const [selection, setSelection] = useState({ scope: selectionScope, managing: false, ids: [] as string[] });
    const [clearing, setClearing] = useState(false);
    const confirming = useRef(false);
    useEffect(() => {
        setSelection((current) => (current.scope === selectionScope ? current : { scope: selectionScope, managing: false, ids: [] }));
    }, [selectionScope]);
    const managing = selection.scope === selectionScope && selection.managing;
    const eligibleIds = tasks.filter(canClearTaskHistory).map((task) => task.id);
    const selectedIds = managing ? selection.ids.filter((id) => eligibleIds.includes(id)) : [];

    const confirmClear = (ids: string[]) => {
        if (!ids.length || clearing || confirming.current) return;
        confirming.current = true;
        modal.confirm({
            title: ids.length === 1 ? "清理这条任务记录？" : `清理 ${ids.length} 条任务记录？`,
            content: "仅从任务历史移除，不删除作品素材，也不影响账务记录。进行中的任务会自动跳过。",
            okText: "清理记录",
            cancelText: "取消",
            okButtonProps: { danger: true },
            onCancel: () => {
                confirming.current = false;
            },
            onOk: async () => {
                setClearing(true);
                try {
                    if (!userId || useUserStore.getState().user?.id !== userId) throw new Error("账号已切换，请重新选择任务");
                    const result = await clearGenerationTaskHistory(ids);
                    await queryClient.cancelQueries({ queryKey: ["workbench-tasks", userId] });
                    onCleared(result.clearedIds);
                    setSelection((current) => ({ ...current, ids: current.ids.filter((id) => !result.clearedIds.includes(id)) }));
                    void queryClient.invalidateQueries({ queryKey: ["workbench-tasks", userId] });
                    if (result.skipped.length) message.warning(`已清理 ${result.clearedIds.length} 条，${result.skipped.length} 条进行中或不可用的任务已跳过`);
                    else message.success(`已清理 ${result.clearedIds.length} 条任务记录`);
                    confirming.current = false;
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "清理任务记录失败");
                    throw error;
                } finally {
                    setClearing(false);
                }
            },
        });
    };

    return {
        managing,
        clearing,
        selectedIds,
        eligibleCount: eligibleIds.length,
        toggleManaging: () => setSelection({ scope: selectionScope, managing: !managing, ids: [] }),
        toggle: (id: string) => setSelection({ scope: selectionScope, managing: true, ids: selectedIds.includes(id) ? selectedIds.filter((value) => value !== id) : [...selectedIds, id] }),
        selectAll: (checked: boolean) => setSelection({ scope: selectionScope, managing: true, ids: checked ? eligibleIds : [] }),
        clearSelected: () => confirmClear(selectedIds),
        clearTask: (task: GenerationTask) => {
            if (canClearTaskHistory(task)) confirmClear([task.id]);
        },
    };
}

export type TaskHistoryCleanup = ReturnType<typeof useTaskHistoryCleanup>;
