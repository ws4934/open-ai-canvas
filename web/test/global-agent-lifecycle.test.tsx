import { afterAll, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { Window } from "happy-dom";
import { setActiveUserScope } from "../src/lib/user-scope";
import { useAgentConversationScope } from "../src/hooks/use-agent-conversation-scope";

const dom = new Window({ url: "http://localhost/" });
const originals = new Map<string, PropertyDescriptor | undefined>();
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, IS_REACT_ACT_ENVIRONMENT: true })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act, StrictMode } = await import("react");
const { createRoot } = await import("react-dom/client");

// Keep the production layout and its lazy boundary; isolate unrelated route pages and network work.
const dir = mkdtempSync(join(tmpdir(), "global-agent-layout-"));
const stub = join(dir, "dependencies.js");
writeFileSync(
    stub,
    `
import React from ${JSON.stringify(import.meta.resolve("react"))};
export const fixture = { pathname: "/create", user: { id: "account-a" }, mounts: 0, unmounts: 0 };
export const useLocation = () => ({ pathname: fixture.pathname });
export const useUserStore = selector => selector(fixture);
export const ConfigProvider = ({ children }) => React.createElement(React.Fragment, null, children);
export const AppWorkspaceShell = ConfigProvider;
export const cn = (...names) => names.filter(Boolean).join(" ");
export const isSpatialWorkbenchPath = () => false;
export const getWorkspaceAntThemeConfig = () => ({});
export const useWorkspaceButtonFeedback = () => {};
export function CanvasCloudAgentPanel() { React.useEffect(() => { fixture.mounts++; return () => { fixture.unmounts++; }; }, []); return React.createElement("div", { "data-agent": true }); }
`,
);
const layoutSource = readFileSync(new URL("../src/layouts/user-layout.tsx", import.meta.url), "utf8")
    .replace(/^import "@\/styles\/[^"\n]+";\n/gm, "")
    .replace(/"(?:react-router|antd|@\/[^"\n]+)"/g, JSON.stringify(stub))
    .replace('from "react"', `from ${JSON.stringify(import.meta.resolve("react"))}`);
const transpiler = new Bun.Transpiler({ loader: "tsx", tsconfig: { compilerOptions: { jsx: "react" } } });
writeFileSync(join(dir, "layout.js"), `import React from ${JSON.stringify(import.meta.resolve("react"))};\n` + transpiler.transformSync(layoutSource));
const { default: UserLayout } = await import(join(dir, "layout.js"));
const { fixture } = await import(stub);

afterAll(() => {
    rmSync(dir, { recursive: true, force: true });
    dom.happyDOM.abort();
    for (const [key, value] of originals) {
        if (value) Object.defineProperty(globalThis, key, value);
        else Reflect.deleteProperty(globalThis, key);
    }
});

test("one account keeps its Agent mounted across product, canvas and admin routes; another account gets a new instance", async () => {
    const host = document.createElement("div");
    document.body.append(host);
    const root = createRoot(host);
    const render = async () => {
        await act(async () => {
            root.render(
                <UserLayout>
                    <div>page</div>
                </UserLayout>,
            );
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    };
    try {
        await render();
        expect(fixture.mounts).toBe(1);
        for (const pathname of ["/canvas/a", "/admin/settings", "/create", "/canvas/b"]) {
            fixture.pathname = pathname;
            await render();
            expect(host.querySelectorAll("[data-agent]")).toHaveLength(1);
            expect(fixture.mounts).toBe(1);
            expect(fixture.unmounts).toBe(0);
        }
        fixture.user = { id: "account-b" };
        await render();
        expect(fixture.mounts).toBe(2);
        expect(fixture.unmounts).toBe(1);
    } finally {
        await act(async () => root.unmount());
        host.remove();
    }
});

test("late continuations stop on conversation changes, account switches before rendering, and unmount", async () => {
    setActiveUserScope("account-a");
    let guard!: ReturnType<typeof useAgentConversationScope>;
    function Probe({ conversation }: { conversation: string }) {
        guard = useAgentConversationScope(conversation);
        return null;
    }
    const host = document.createElement("div");
    document.body.append(host);
    const root = createRoot(host);
    await act(async () =>
        root.render(
            <StrictMode>
                <Probe conversation="first" />
            </StrictMode>,
        ),
    );
    expect(guard.isCurrentScope("first")).toBe(true);
    await act(async () =>
        root.render(
            <StrictMode>
                <Probe conversation="second" />
            </StrictMode>,
        ),
    );
    expect(guard.isCurrentScope("first")).toBe(false);
    expect(guard.isCurrentScope("second")).toBe(true);
    setActiveUserScope("account-b");
    expect(guard.isCurrentScope("second")).toBe(false);
    setActiveUserScope("account-a");
    await act(async () => root.unmount());
    expect(guard.isCurrentScope("second")).toBe(false);
    host.remove();
});

test("run controls and old node references do not borrow a different page's canvas", () => {
    const source = readFileSync(new URL("../src/components/canvas/canvas-cloud-agent-panel.tsx", import.meta.url), "utf8");
    expect(source).toContain("const otherAgentRunning = !running &&");
    expect(source).toContain("approval && run?.canvasId === canvasId ? agentApprovalTargetGenerating");
    expect(source).toContain("onFocusNode={run && run.canvasId !== canvasId ? undefined : onFocusNode}");
    expect(source).toContain("pending && pending.canvasId !== canvasId");
    expect(source).toContain("prefillRequest.canvasId !== canvasId");
    expect(source).toContain("const key = `${canvasId}:${prefillRequest.id}`");
    expect(source).toMatch(/accepted = true;\s*if \(isCurrentScope\(scope\)\) \{/);
    expect(source).toContain("clearCloudAgentPendingSubmission(activeConversationId, accountScope)");
});
