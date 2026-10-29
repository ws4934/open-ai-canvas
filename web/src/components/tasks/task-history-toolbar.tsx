import { Button } from "antd";
import { Trash2 } from "lucide-react";

import { Checkbox } from "@/components/ui/base/checkbox/checkbox";
import type { TaskHistoryCleanup } from "@/hooks/use-task-history-cleanup";

export function TaskHistoryToolbar({ cleanup }: { cleanup: TaskHistoryCleanup }) {
    return (
        <div className="flex flex-wrap items-center gap-3" aria-label="任务历史管理">
            {cleanup.managing ? (
                <>
                    <Checkbox
                        checked={cleanup.eligibleCount > 0 && cleanup.selectedIds.length === cleanup.eligibleCount}
                        indeterminate={cleanup.selectedIds.length > 0 && cleanup.selectedIds.length < cleanup.eligibleCount}
                        disabled={!cleanup.eligibleCount || cleanup.clearing}
                        onChange={(event) => cleanup.selectAll(event.target.checked)}
                    >
                        全选本页
                    </Checkbox>
                    <Button danger size="small" icon={<Trash2 className="size-3.5" />} disabled={!cleanup.selectedIds.length} loading={cleanup.clearing} onClick={cleanup.clearSelected}>
                        清理所选{cleanup.selectedIds.length ? ` (${cleanup.selectedIds.length})` : ""}
                    </Button>
                </>
            ) : null}
            <Button size="small" disabled={cleanup.clearing || (!cleanup.managing && !cleanup.eligibleCount)} aria-pressed={cleanup.managing} onClick={cleanup.toggleManaging}>
                {cleanup.managing ? "取消管理" : "批量管理"}
            </Button>
        </div>
    );
}
