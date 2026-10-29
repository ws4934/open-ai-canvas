import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { queryGenerationTask, type GenerationTask } from "@/services/api/task-center";
import { listWorkbenchActiveTasks, listWorkbenchTasks, type WorkbenchKind, type WorkbenchTaskPage } from "@/services/api/workbench";
import { useUserStore } from "@/stores/use-user-store";

export const WORKBENCH_HISTORY_PAGE_SIZE = 20;
const isActive = (task: GenerationTask) => task.status === "queued" || task.status === "running";

export const workbenchTaskKeys = {
    history: (userId: string, kind: WorkbenchKind) => ["workbench-tasks", userId, kind, "history"] as const,
    historyPage: (userId: string, kind: WorkbenchKind, page: number, query: string) => [...workbenchTaskKeys.history(userId, kind), page, query] as const,
    active: (userId: string, kind: WorkbenchKind) => ["workbench-tasks", userId, kind, "active"] as const,
    selected: (userId: string, kind: WorkbenchKind, taskId?: string) => ["workbench-tasks", userId, kind, "selected", taskId] as const,
};

export function workbenchHistoryQueryOptions(userId: string, kind: WorkbenchKind, page: number, query: string) {
    return {
        queryKey: workbenchTaskKeys.historyPage(userId, kind, page, query),
        queryFn: ({ signal }: { signal: AbortSignal }) => listWorkbenchTasks(kind, { page, pageSize: WORKBENCH_HISTORY_PAGE_SIZE, query }, signal),
        enabled: Boolean(userId),
        refetchOnWindowFocus: "always" as const,
    };
}

export function workbenchActiveQueryOptions(userId: string, kind: WorkbenchKind) {
    return {
        queryKey: workbenchTaskKeys.active(userId, kind),
        queryFn: ({ signal }: { signal: AbortSignal }) => listWorkbenchActiveTasks(kind, signal),
        enabled: Boolean(userId),
        refetchOnWindowFocus: "always" as const,
        refetchInterval: (query: { state: { data?: GenerationTask[] } }) => (query.state.data?.length ? 5_000 : 30_000),
    };
}

export function useWorkbenchTasks(kind: WorkbenchKind, watchedTask: GenerationTask | null) {
    const userId = useUserStore((state) => state.user?.id || "");
    const queryClient = useQueryClient();
    const [filter, setFilter] = useState({ page: 1, query: "" });
    const search = filter.query.trim();
    const debouncedSearch = useDebouncedValue(search);
    const history = useQuery({ ...workbenchHistoryQueryOptions(userId, kind, filter.page, search), enabled: Boolean(userId) && search === debouncedSearch });
    const active = useQuery(workbenchActiveQueryOptions(userId, kind));
    const [featuredTask, setFeaturedTask] = useState<GenerationTask | null>(null);
    const previousActiveIds = useRef<string[] | null>(null);

    useEffect(() => {
        const data = history.data;
        if (!data) return;
        if (data.page !== filter.page) {
            queryClient.setQueryData(workbenchTaskKeys.historyPage(userId, kind, data.page, search), data);
            setFilter((current) => (current.page === filter.page && current.query.trim() === search ? { ...current, page: data.page } : current));
        }
        if (data.page === 1 && !search) setFeaturedTask(data.tasks.find((task) => task.status === "succeeded") || null);
    }, [filter.page, history.data, kind, queryClient, search, userId]);

    useEffect(() => {
        if (!active.data) return;
        const ids = active.data.map((task) => task.id);
        if (previousActiveIds.current?.some((id) => !ids.includes(id))) void queryClient.invalidateQueries({ queryKey: workbenchTaskKeys.history(userId, kind) });
        previousActiveIds.current = ids;
    }, [active.data, kind, queryClient, userId]);

    const watchedSummary = active.data?.find((task) => task.id === watchedTask?.id);
    // 活动列表只返回未结束任务；选中的任务消失时，单独取一次完整终态，不能把它误当失败。
    const missingWatchedTask = watchedTask && isActive(watchedTask) && active.data && !watchedSummary;
    const completed = useQuery({
        queryKey: workbenchTaskKeys.selected(userId, kind, watchedTask?.id),
        queryFn: ({ signal }) => queryGenerationTask(watchedTask!.id, { signal }),
        enabled: Boolean(userId && missingWatchedTask),
        staleTime: 0,
        refetchInterval: (query) => (query.state.data && isActive(query.state.data) ? 5_000 : false),
    });

    const rememberTask = useCallback(
        async (task: GenerationTask) => {
            await Promise.all([queryClient.cancelQueries({ queryKey: workbenchTaskKeys.active(userId, kind) }), queryClient.cancelQueries({ queryKey: workbenchTaskKeys.selected(userId, kind, task.id) })]);
            queryClient.setQueryData(workbenchTaskKeys.selected(userId, kind, task.id), task);
            queryClient.setQueryData<GenerationTask[]>(workbenchTaskKeys.active(userId, kind), (current) => {
                const rest = (current || []).filter((item) => item.id !== task.id);
                return isActive(task) ? [task, ...rest] : rest;
            });
            await queryClient.invalidateQueries({ queryKey: workbenchTaskKeys.history(userId, kind) });
        },
        [kind, queryClient, userId],
    );

    const refresh = useCallback(async () => {
        await Promise.all([history.refetch(), active.refetch(), ...(missingWatchedTask ? [completed.refetch()] : [])]);
    }, [active.refetch, completed.refetch, history.refetch, missingWatchedTask]);

    const forgetTasks = (ids: string[]) => {
        setFeaturedTask((current) => (current && ids.includes(current.id) ? null : current));
        queryClient.setQueriesData<WorkbenchTaskPage>({ queryKey: workbenchTaskKeys.history(userId, kind) }, (current) =>
            current ? { ...current, tasks: current.tasks.filter((task) => !ids.includes(task.id)), total: Math.max(0, current.total - current.tasks.filter((task) => ids.includes(task.id)).length) } : current,
        );
        for (const id of ids) queryClient.removeQueries({ queryKey: workbenchTaskKeys.selected(userId, kind, id), exact: true });
    };

    return {
        activeTasks: active.data || [],
        historyTasks: history.data?.tasks || [],
        total: history.data?.total ?? 0,
        page: history.data?.page ?? filter.page,
        pageSize: WORKBENCH_HISTORY_PAGE_SIZE,
        query: filter.query,
        setQuery: (query: string) => setFilter({ page: 1, query }),
        setPage: (page: number) => setFilter((current) => ({ ...current, page })),
        loading: history.isPending || search !== debouncedSearch,
        refreshing: history.isFetching || active.isFetching || completed.isFetching,
        historyError: history.error,
        activeError: active.error || completed.error,
        featuredTask,
        observedTask: watchedSummary || completed.data,
        rememberTask,
        forgetTasks,
        refresh,
    };
}
