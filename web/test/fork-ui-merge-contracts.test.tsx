import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { renderToStaticMarkup } from "react-dom/server";
import { ImageSizePresetsEditor } from "../src/components/image-size-presets-editor";
import { DEFAULT_CANVAS_APPEARANCE } from "../src/lib/canvas/agent-appearance";
import { defaultImageCapabilityConfig } from "../src/lib/model-capabilities";
import { imagePresetForRatio } from "../src/lib/image-size-presets";

const source = (path: string) => readFileSync(new URL(`../src/${path}`, import.meta.url), "utf8");

test("image/video workbench retains two columns and the unified results gallery alongside upstream prompt switching", () => {
    const page = source("pages/workbench/image-workbench-page.tsx");
    const css = source("pages/workbench/workbench.css");
    expect(page).toContain('aria-label="生成参数"');
    expect(page).toContain('aria-label="作品"');
    expect(page).toContain('aria-label="历史作品"');
    expect(page).toContain("switchToolPrompt(next)");
    expect(page.match(/className="workbench-submit"/g)).toHaveLength(1);
    expect(css).toMatch(/\.workbench-columns\s*\{[^}]*grid-template-columns:\s*minmax\(360px, 420px\) minmax\(0, 1fr\)/);
    expect(css).not.toContain(".workbench-history-column");
});

test("compact two-pane model selection keeps the local rail width", () => {
    const css = source("styles/workspace-product.css");
    expect(css).toMatch(/\.creation-model-picker-surface[^{}]*\.canvas-model-picker-two-pane\s*\{[^}]*grid-template-columns:\s*220px minmax\(0, 1fr\)/);
});

test("size presets render aligned rows and collapse unused protocol defaults", () => {
    const profile = defaultImageCapabilityConfig();
    const preset = imagePresetForRatio("1k", "1:1");
    profile.size = { ...profile.size, parameter: "size", presets: [preset], values: ["auto", preset.size, "2048x1024"], default: preset.size };
    const html = renderToStaticMarkup(<ImageSizePresetsEditor profile={profile} tiers={["1k"]} ratios={["1:1"]} onChange={() => {}} />);
    expect(html).toContain('class="image-size-preset-row is-selected"');
    expect(html).toContain('aria-label="1K 1:1 宽度像素"');
    expect(html).toContain('<details class="image-size-default-more">');
    expect(html).toContain("其他支持尺寸（1）");
    const css = source("components/image-size-picker.css");
    expect(css).toMatch(/\.image-size-preset-option\s*\{\s*display:\s*inline-flex/);
});

test("small Agent appearance keeps title clearance and the scoped Live2D preview priority", () => {
    expect(DEFAULT_CANVAS_APPEARANCE.avatarHeight).toBe(150);
    const css = source("components/canvas/canvas-cloud-agent.css");
    expect(css).toMatch(/\.agent-panel-header\s*\{[^}]*padding:\s*var\(--space-3\) var\(--space-4\) var\(--space-2\) var\(--space-6\)/);
    const editor = source("pages/admin/settings/components/canvas-appearance-editor.tsx");
    expect(editor).toContain("hidden={!showLive2DPreview}");
    expect(editor).toContain("renderPriority={1}");
});

test("admin feedback and memory lists retain the shared toolbar and table surface", () => {
    for (const path of ["pages/admin/components/agent-lessons-panel.tsx", "pages/admin/feedbacks/feedbacks-page.tsx"]) {
        const page = source(path);
        expect(page).toContain("<AdminDataTable");
        expect(page).toContain("toolbar={");
        expect(page).toContain("<AdminSearchInput");
    }
});

test("docked Agent layout retains the admin boundary and bound-run controls", () => {
    const layout = source("layouts/user-layout.tsx");
    expect(layout).toContain("adminWorkspace ? (");
    expect(layout).toContain("paddingRight: agentDock.width");
    const panel = source("components/canvas/canvas-cloud-agent-panel.tsx");
    expect(panel).toContain("const otherAgentRunning = !running && Boolean(canvasAgent && canvasAgent.runId !== run?.id)");
    expect(panel).toContain("run?.canvasId === canvasId ? approvalTargetGenerating : undefined");
    expect(panel).toContain('data-agent-resize="dock"');
});

test("debounced Agent cache writes keep account and conversation scope guards", () => {
    const panel = source("components/canvas/canvas-cloud-agent-panel.tsx");
    expect(panel).toContain("conversationAutosaveRef.current = { timer: setTimeout(save, delay), save, since }");
    expect(panel).toContain("if (!accountIsCurrent() || !isCurrentScope(activeConversationId)) return");
    expect(panel).toMatch(/\(\) => \(canvasId \? \(?canvasEditLeases\.agent\(canvasId\) \?\? null\)? : null\)/);
});
