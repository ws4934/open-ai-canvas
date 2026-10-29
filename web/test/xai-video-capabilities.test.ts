import { expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { VideoSettingsPanel } from "@/components/video-settings-panel";
import { canvasThemes } from "@/lib/canvas-theme";
import { defaultModelCapabilityConfig, normalizeModelCapabilityConfig } from "@/lib/model-capabilities";
import { defaultConfig, type AiConfig } from "@/stores/use-config-store";

test("xAI 两代与 Lite 的默认能力不混用", () => {
    const old = defaultModelCapabilityConfig("xai-video", "grok-imagine-video").video!;
    const full = defaultModelCapabilityConfig("xai-video", "grok-imagine-video-1.5").video!;
    const lite = defaultModelCapabilityConfig("xai-video", "grok-imagine-video-1.5-lite").video!;
    expect(old.duration.default).toBe(8);
    expect(old.references.maxImages).toBe(7);
    expect(old.operations).toContain("extend");
    expect(old.resolutions).not.toContain("1080p");
    expect(full.references.maxImages).toBe(16);
    expect(full.ratios).toHaveLength(9);
    expect(full.resolutions).toContain("1080p");
    expect(full.resolutions).not.toContain("1440p");
    expect(full.generateAudio).toEqual({ supported: true, default: true, fixed: true });
    expect(full.operations).not.toContain("extend");
    expect(full.references.maxAudios).toBe(0);
    expect(lite.references.maxImages).toBe(1);
    expect(lite.operations).not.toContain("reference_to_video");
});

test("xAI 原生音频提示在能力归一化后保留", () => {
    const capability = defaultModelCapabilityConfig("xai-video", "grok-imagine-video-1.5");
    expect(normalizeModelCapabilityConfig(JSON.parse(JSON.stringify(capability))).video!.generateAudio.fixed).toBe(true);
});

test("真实视频设置面板只读展示 xAI 原生声音，不沿用旧的静音值", () => {
    const model = "grok-imagine-video-1.5";
    const config: AiConfig = {
        ...defaultConfig,
        model: `xai-test::${model}`,
        videoGenerateAudio: "false",
        channels: [
            {
                id: "xai-test",
                name: "xAI",
                baseUrl: "https://api.x.ai",
                apiKey: "",
                apiFormat: "openai",
                interfaceType: "xai-video",
                models: [model],
                modelCosts: [{ model, protocol: "xai-video", capability: "video", billingMode: "per_second", unitPriceMicrocredits: 0 }],
            },
        ],
    };
    for (const theme of [canvasThemes.light, canvasThemes.dark]) {
        const html = renderToStaticMarkup(createElement(VideoSettingsPanel, { config, theme, onConfigChange: () => {} }));
        const audioSwitch = html.match(/<button(?=[^>]*aria-label="原生声音")[^>]*>/)?.[0];
        expect(audioSwitch).toContain('aria-checked="true"');
        expect(audioSwitch).toContain("disabled");
        expect(html).toContain("1080P");
        expect(html).not.toContain("添加水印");
    }
});
