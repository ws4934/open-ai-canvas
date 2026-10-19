import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { App } from "antd";
import { renderToStaticMarkup } from "react-dom/server";
import { Window } from "happy-dom";
import { DEFAULT_CANVAS_APPEARANCE, type CanvasAppearance } from "../src/lib/canvas/agent-appearance";
import { CanvasAppearanceEditor } from "../src/pages/admin/settings/components/canvas-appearance-editor";

const withBothAvatars: CanvasAppearance = {
    ...DEFAULT_CANVAS_APPEARANCE,
    live2dResourceId: "existing-live2d",
    live2dEntry: "model.model3.json",
    avatarBundleResourceId: "new-animation-bundle",
};

function renderEditor(avatarType: CanvasAppearance["avatarType"]) {
    return renderToStaticMarkup(
        <App>
            <CanvasAppearanceEditor value={{ ...withBothAvatars, avatarType }} onChange={() => {}} disabled={false} onUploading={() => {}} />
        </App>,
    );
}

test("animation-bundle preview is not replaced by a retained Live2D reference", () => {
    const html = renderEditor("bundle");
    expect(html).toContain("当前状态动画");
    expect(html).not.toContain("等待模型预览就绪");
    expect(html).toContain("移除模型引用");
    const dom = new Window();
    try {
        dom.document.body.innerHTML = html;
        const preview = dom.document.querySelector('[aria-label="Agent 形象预览"]')!;
        const hiddenModel = preview.querySelector("[hidden]")!;
        expect(hiddenModel.textContent).toContain("正在加载 Live2D 模型");
        expect(dom.document.querySelector('input[value="live2d"]')?.hasAttribute("disabled")).toBe(true);
        hiddenModel.remove();
        expect(preview.textContent).not.toContain("正在加载 Live2D 模型");
    } finally {
        dom.close();
    }
});

test("switching back to Live2D still previews the retained model", () => {
    const html = renderEditor("live2d");
    expect(html).toContain("正在加载 Live2D 模型");
    expect(html).toContain("等待模型预览就绪");
});

test("newly imported Live2D models retain the existing preview-before-enable flow", () => {
    const html = renderEditor("orb");
    expect(html).toContain("正在加载 Live2D 模型");
    expect(html).toContain("等待模型预览就绪");
});

for (const state of ["cancelled", "rejected"]) {
    test(`${state} preview uses the launcher's single-play terminal-state contract`, () => {
        const source = readFileSync(new URL("../src/pages/admin/settings/components/canvas-appearance-editor.tsx", import.meta.url), "utf8");
        const terminalStates = source.match(/loop=\{!(\[[^\]]*\])\.includes\(selectedAvatarState\)\}/)?.[1];
        expect(JSON.parse(terminalStates ?? "[]")).toContain(state);
    });
}
