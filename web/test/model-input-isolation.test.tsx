import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import manifest from "../../plugin-packages/xai-video/manifest.json";
import { ModelCapabilityEditor } from "../src/components/model-capability-editor";
import { defaultModelCapabilityConfig } from "../src/lib/model-capabilities";
import { projectInputConfig, type InputSchema } from "../src/lib/model-inputs";
import type { ModelProtocolDefinition } from "../src/lib/model-protocols";
import { changeChannelModelProtocol, initialChannelModelValues } from "../src/pages/admin/components/channel-model-editor-form";
import { prepareBackendGenerationTask } from "../src/services/api/generation-task";
import { createModelChannel, defaultConfig, encodeChannelModel } from "../src/stores/use-config-store";
import { usePluginStore } from "../src/stores/use-plugin-store";
import { RUNNINGHUB_PLUGIN_ID } from "../src/lib/plugins/builtin/workflows";

const schema = manifest.contributes.providers[0].inputSchema as unknown as InputSchema;

describe("model input protocol isolation", () => {
    test("a workflow does not inherit material roles from the previously selected xAI model", async () => {
        const runtimeStatuses = usePluginStore.getState().runtimeStatuses;
        usePluginStore.setState({ runtimeStatuses: { ...runtimeStatuses, [RUNNINGHUB_PLUGIN_ID]: "enabled" } });
        try {
            const profile = projectInputConfig(defaultModelCapabilityConfig("xai-video"), schema.templates![0].config);
            const channel = createModelChannel({ id: "qa-workflow-xai", models: ["grok-imagine-video-1.5"], modelCosts: [{ model: "grok-imagine-video-1.5", capability: "video", protocol: "xai-video", capabilityConfig: profile }] });
            const task = await prepareBackendGenerationTask({
                mode: "video",
                prompt: "a scene",
                config: {
                    ...defaultConfig,
                    channels: [channel],
                    model: encodeChannelModel(channel.id, "grok-imagine-video-1.5"),
                    taskWorkflowProvider: "runninghub",
                    inputParameters: "{",
                    inputMediaRoles: "{",
                    runningHub: {
                        ...defaultConfig.runningHub,
                        enabled: true,
                        baseUrl: "https://runninghub.example",
                        apiKey: "test-placeholder",
                        workflowId: "qa-workflow",
                        capability: "video",
                        workflows: [{ workflowId: "qa-workflow", capability: "video" }],
                    },
                },
                referenceImages: [{ id: "pin", name: "pin", type: "image/png", dataUrl: "", url: "https://media.example/pin.png", role: "reference_image" }],
            });
            expect(task.provider).toBe("runninghub");
            expect(task.input?.config).not.toHaveProperty("inputParameters");
            expect(task.input?.config).not.toHaveProperty("inputMediaRoles");
            expect(task.input?.referenceImages).toMatchObject([{ id: "pin", role: "reference_image" }]);
        } finally {
            usePluginStore.setState({ runtimeStatuses });
        }
    });
    test("protocols without inputSchema retain their original reference and parameter editors", () => {
        for (const [capability, protocol, expected] of [
            ["text", "openai", "上下文"],
            ["image", "openai-image", "蒙版编辑"],
            ["video", "minimax-video", "图片引用"],
        ] as const) {
            const html = renderToStaticMarkup(<ModelCapabilityEditor protocol={protocol} capability={capability} value={defaultModelCapabilityConfig(protocol)} section="references" />);
            expect(html).not.toContain('class="model-input-config-editor"');
            expect(html).not.toContain("能力模板");
            expect(html).toContain(capability === "text" ? "通用限制" : expected);
        }
    });

    test("switching away from xAI replaces its config rather than carrying it into another protocol", () => {
        const protocols: ModelProtocolDefinition[] = [
            { value: "xai-video", label: "xAI", capability: "video", create: "POST /videos", contentType: "application/json", media: "url", inputSchema: schema },
            { value: "minimax-video", label: "MiniMax", capability: "video", create: "POST /video_generation", contentType: "application/json", media: "url" },
        ];
        const draft = initialChannelModelValues(null, protocols);
        draft.capability = "video";
        draft.protocol = "xai-video";
        draft.capabilityConfig = projectInputConfig(defaultModelCapabilityConfig("xai-video"), schema.templates![0].config);
        draft.priceTiers[0].unitPrice = 13;
        const next = changeChannelModelProtocol(draft, "minimax-video", protocols);
        expect(next.capabilityConfig?.inputConfig).toBeUndefined();
        expect(next.capabilityConfig).toEqual(defaultModelCapabilityConfig("minimax-video"));
        expect(next.priceTiers[0].unitPrice).toBe(13);
        expect(draft.capabilityConfig?.inputConfig).toBeDefined();
    });

    test("other video tasks do not parse or apply stale xAI parameters and material assignments", async () => {
        const channel = createModelChannel({
            id: "qa-other-protocol",
            models: ["MiniMax-H3"],
            modelCosts: [{ model: "MiniMax-H3", capability: "video", protocol: "minimax-video", capabilityConfig: defaultModelCapabilityConfig("minimax-video", "MiniMax-H3") }],
        });
        const model = encodeChannelModel(channel.id, "MiniMax-H3");
        for (const inputMediaRoles of ["{", JSON.stringify({ pin: { role: "keyframe", inputFields: { timestamp_s: 2 } } })]) {
            const task = await prepareBackendGenerationTask({
                mode: "video",
                prompt: "a scene",
                config: { ...defaultConfig, channels: [channel], model, videoModel: model, inputParameters: "{", inputMediaRoles },
                referenceImages: [{ id: "pin", name: "pin", type: "image/png", dataUrl: "", url: "https://media.example/pin.png", role: "first_frame" }],
            });
            expect(task.input?.referenceImages).toMatchObject([{ id: "pin", role: "first_frame" }]);
            expect((task.input?.referenceImages as Record<string, unknown>[])[0]).not.toHaveProperty("inputFields");
            expect(task.input?.config).not.toHaveProperty("inputParameters");
            expect(task.input?.config).not.toHaveProperty("inputMediaRoles");
        }
    });
});
