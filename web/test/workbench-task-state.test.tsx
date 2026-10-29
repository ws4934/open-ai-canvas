import { afterAll, expect, test } from "bun:test";
import { Window } from "happy-dom";

const dom = new Window({ url: "http://localhost/" });
const originals = new Map<string, PropertyDescriptor | undefined>();
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, IS_REACT_ACT_ENVIRONMENT: true })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act, useState } = await import("react");
const { createRoot } = await import("react-dom/client");
const { QueryClient, QueryClientProvider } = await import("@tanstack/react-query");
const { apiClient } = await import("@/services/api/request");
const { useUserStore } = await import("@/stores/use-user-store");
const { useWorkbenchTasks, workbenchTaskKeys } = await import("@/pages/workbench/use-workbench-tasks");
const { WorkbenchTaskThumbnail } = await import("@/pages/workbench/workbench-task-thumbnail");
type GenerationTask = import("@/services/api/task-center").GenerationTask;

afterAll(() => {
    dom.happyDOM.abort();
    for (const [key, value] of originals) {
        if (value) Object.defineProperty(globalThis, key, value);
        else Reflect.deleteProperty(globalThis, key);
    }
});

test("视频封面在进入可视区前没有 src，进入后才请求元数据并停止观察", async () => {
    const original = Object.getOwnPropertyDescriptor(globalThis, "IntersectionObserver");
    let notify!: (entries: { isIntersecting: boolean }[]) => void;
    let disconnected = false;
    let rootMargin = "";
    Object.defineProperty(globalThis, "IntersectionObserver", {
        configurable: true,
        writable: true,
        value: class {
            constructor(callback: typeof notify, options: { rootMargin: string }) {
                notify = callback;
                rootMargin = options.rootMargin;
            }
            observe() {}
            disconnect() {
                disconnected = true;
            }
        },
    });
    const host = document.createElement("div");
    const root = createRoot(host);
    try {
        await act(async () => root.render(<WorkbenchTaskThumbnail url="/api/resources/qa-video/file" video />));
        expect(host.querySelector("video")?.getAttribute("src")).toBeNull();
        expect(rootMargin).toBe("200px");
        await act(async () => notify([{ isIntersecting: false }]));
        expect(host.querySelector("video")?.getAttribute("src")).toBeNull();
        await act(async () => notify([{ isIntersecting: true }]));
        expect(host.querySelector("video")?.getAttribute("src")).toBe("/api/resources/qa-video/file");
        expect(host.querySelector("video")?.preload).toBe("metadata");
        expect(disconnected).toBe(true);
    } finally {
        await act(async () => root.unmount());
        if (original) Object.defineProperty(globalThis, "IntersectionObserver", original);
        else Reflect.deleteProperty(globalThis, "IntersectionObserver");
    }
});

const task = (status: GenerationTask["status"]): GenerationTask => ({ id: "same-id", type: "canvas_image", status, prompt: "测试图片", attempts: 1, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" });

test("同 ID 人工重试替换旧失败缓存，完成后读取新终态，清理移除当前展示", async () => {
    const oldUser = useUserStore.getState().user;
    useUserStore.setState({ user: { ...oldUser, id: "state-test" } as NonNullable<typeof oldUser> });
    const original = apiClient.request;
    let backendActive: GenerationTask[] = [];
    let backendHistory: GenerationTask[] = [task("failed")];
    let backendDetail = task("succeeded");
    const calls: string[] = [];
    apiClient.request = (async (config: { url?: string; params?: { activeOnly?: string } }) => {
        calls.push(config.url || "");
        const data =
            config.url === "/workbench/tasks"
                ? { tasks: config.params?.activeOnly ? backendActive : backendHistory, total: config.params?.activeOnly ? backendActive.length : backendHistory.length, page: 1, pageSize: config.params?.activeOnly ? 100 : 20, hasMore: false }
                : backendDetail;
        return { status: 200, data: { code: 0, data, msg: "" } };
    }) as typeof apiClient.request;
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    client.setQueryData(workbenchTaskKeys.selected("state-test", "image", "same-id"), task("failed"));
    let history!: ReturnType<typeof useWorkbenchTasks>;
    let setWatched!: (task: GenerationTask) => void;
    function View() {
        const [watched, update] = useState<GenerationTask | null>(null);
        setWatched = update;
        history = useWorkbenchTasks("image", watched);
        return null;
    }
    const host = document.createElement("div");
    const root = createRoot(host);
    const flush = () => act(() => new Promise((resolve) => setTimeout(resolve, 20)));
    try {
        await act(async () =>
            root.render(
                <QueryClientProvider client={client}>
                    <View />
                </QueryClientProvider>,
            ),
        );
        await flush();
        expect(history.historyTasks[0].status).toBe("failed");
        backendActive = [task("queued")];
        backendHistory = [];
        await act(async () => {
            await history.rememberTask(backendActive[0]);
            setWatched(backendActive[0]);
        });
        expect(client.getQueryData<GenerationTask>(workbenchTaskKeys.selected("state-test", "image", "same-id"))?.status).toBe("queued");
        backendActive = [];
        backendHistory = [backendDetail];
        await act(async () => {
            await client.invalidateQueries({ queryKey: workbenchTaskKeys.active("state-test", "image") });
        });
        await flush();
        expect(history.observedTask?.status).toBe("succeeded");
        expect(calls.filter((url) => url === "/tasks/same-id")).toHaveLength(1);
        await flush();
        expect(history.historyTasks[0].status).toBe("succeeded");
        expect(history.featuredTask?.id).toBe("same-id");
        await act(async () => {
            history.forgetTasks(["same-id"]);
        });
        expect(history.historyTasks).toHaveLength(0);
        expect(history.featuredTask).toBeNull();
    } finally {
        await act(async () => root.unmount());
        await client.cancelQueries();
        client.clear();
        apiClient.request = original;
        useUserStore.setState({ user: oldUser });
        host.remove();
    }
});
