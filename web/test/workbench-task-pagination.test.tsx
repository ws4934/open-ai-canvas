import { describe, expect, test } from "bun:test";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import type { AxiosRequestConfig } from "axios";
import { renderToStaticMarkup } from "react-dom/server";

import { PaginationBar } from "@/components/layout/workspace-page";
import { workbenchActiveQueryOptions, workbenchHistoryQueryOptions, workbenchTaskKeys } from "@/pages/workbench/use-workbench-tasks";
import { WorkbenchTaskThumbnail } from "@/pages/workbench/workbench-task-thumbnail";
import { apiClient } from "@/services/api/request";
import { listWorkbenchActiveTasks, listWorkbenchTasks, type WorkbenchTaskPage } from "@/services/api/workbench";

const page = (number: number, total = 123): WorkbenchTaskPage => ({ page: number, pageSize: 20, total, tasks: [], hasMore: number * 20 < total });
const envelope = (data: unknown) => ({ status: 200, data: { code: 0, data, msg: "" } });

describe("工作台真实服务端分页", () => {
    test("默认每页 20 条，页码/全历史搜索/取消信号直接传给后端", async () => {
        const original = apiClient.request;
        const calls: AxiosRequestConfig[] = [];
        apiClient.request = (async (config: AxiosRequestConfig) => {
            calls.push(config);
            return envelope(page(Number(config.params.page)));
        }) as typeof apiClient.request;
        try {
            await listWorkbenchTasks("image");
            expect(calls[0].params).toEqual({ kind: "image", page: 1, pageSize: 20 });
            const controller = new AbortController();
            const result = await listWorkbenchTasks("video", { page: 7, query: "  很久以前的作品  " }, controller.signal);
            expect(calls[1].params).toEqual({ kind: "video", page: 7, pageSize: 20, q: "很久以前的作品" });
            expect(calls[1].signal).toBe(controller.signal);
            expect(result.page).toBe(7);
            expect(result.total).toBe(123);
        } finally {
            apiClient.request = original;
        }
    });

    test("活动任务独立获取，超过一页时也不混入历史或漏掉其余活动任务", async () => {
        const original = apiClient.request;
        const calls: AxiosRequestConfig[] = [];
        apiClient.request = (async (config: AxiosRequestConfig) => {
            calls.push(config);
            const number = Number(config.params.page);
            return envelope({ ...page(number, 101), pageSize: 100, hasMore: number === 1, tasks: [{ id: `active-${number}`, status: "running" }] });
        }) as typeof apiClient.request;
        try {
            const tasks = await listWorkbenchActiveTasks("image");
            expect(tasks.map((task) => task.id)).toEqual(["active-1", "active-2"]);
            expect(calls.map((call) => call.params)).toEqual([
                { kind: "image", page: 1, pageSize: 100, activeOnly: "true" },
                { kind: "image", page: 2, pageSize: 100, activeOnly: "true" },
            ]);
        } finally {
            apiClient.request = original;
        }
    });

    test("翻页取消旧请求，迟到的上一页不会覆盖当前页", async () => {
        const original = apiClient.request;
        const pending: Array<{ config: AxiosRequestConfig; respond: (value: unknown) => void }> = [];
        apiClient.request = ((config: AxiosRequestConfig) => new Promise((respond) => pending.push({ config, respond }))) as typeof apiClient.request;
        const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
        const observer = new QueryObserver(client, workbenchHistoryQueryOptions("user-a", "image", 1, ""));
        let gotSecond!: (value: unknown) => void;
        const second = new Promise((resolve) => {
            gotSecond = resolve;
        });
        const unsubscribe = observer.subscribe((result) => {
            if (result.data?.page === 2) gotSecond(result.data);
        });
        try {
            expect(pending).toHaveLength(1);
            observer.setOptions(workbenchHistoryQueryOptions("user-a", "image", 2, ""));
            expect(pending[0].config.signal?.aborted).toBe(true);
            expect(pending).toHaveLength(2);
            pending[1].respond(envelope(page(2)));
            await second;
            pending[0].respond(envelope(page(1)));
            await Promise.resolve();
            expect(observer.getCurrentResult().data?.page).toBe(2);
            expect(client.getQueryData(workbenchTaskKeys.historyPage("user-a", "image", 2, ""))).toEqual(page(2));
        } finally {
            unsubscribe();
            await client.cancelQueries();
            client.clear();
            apiClient.request = original;
        }
    });

    test("账号、类型、搜索、页码和活动列表各有缓存键，历史没有轮询", () => {
        const history = workbenchHistoryQueryOptions("user-a", "image", 1, "猫");
        expect(history).not.toHaveProperty("refetchInterval");
        expect(history.refetchOnWindowFocus).toBe("always");
        expect(history.queryKey).not.toEqual(workbenchHistoryQueryOptions("user-b", "image", 1, "猫").queryKey);
        expect(history.queryKey).not.toEqual(workbenchHistoryQueryOptions("user-a", "video", 1, "猫").queryKey);
        expect(history.queryKey).not.toEqual(workbenchHistoryQueryOptions("user-a", "image", 2, "猫").queryKey);
        expect(history.queryKey).not.toEqual(workbenchHistoryQueryOptions("user-a", "image", 1, "狗").queryKey);
        const active = workbenchActiveQueryOptions("user-a", "image");
        expect(active.queryKey).not.toEqual(history.queryKey);
        expect(active.refetchInterval({ state: { data: [] } })).toBe(30_000);
        expect(active.refetchInterval({ state: { data: [{ status: "running" } as never] } })).toBe(5_000);
    });

    test("历史封面懒加载，离屏视频不带 src，翻页控件使用完整总数", () => {
        const image = renderToStaticMarkup(<WorkbenchTaskThumbnail url="/api/resources/image/file" video={false} />);
        expect(image).toContain('loading="lazy"');
        const video = renderToStaticMarkup(<WorkbenchTaskThumbnail url="/api/resources/video/file" video />);
        expect(video).toContain("<video");
        expect(video).not.toContain("/api/resources/video/file");
        const pagination = renderToStaticMarkup(<PaginationBar current={6} pageSize={20} total={123} pageSizeOptions={[20]} onChange={() => {}} />);
        expect(pagination).toContain('aria-label="第 7 页"');
        expect(pagination).toContain("123");
    });
});
