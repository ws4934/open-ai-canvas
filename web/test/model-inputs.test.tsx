import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import manifest from "../../plugin-packages/xai-video/manifest.json";
import { ModelInputConfigEditor } from "../src/components/model-input-config-editor";
import { ModelInputFields, ModelInputMediaAssignments } from "../src/components/model-input-fields";
import { effectiveInputFields, initializeInputConfig, inputDefaults, inputHostPatch, modelInputRequest, projectInputConfig, resolveInputConfig, type InputSchema } from "../src/lib/model-inputs";
import { defaultModelCapabilityConfig } from "../src/lib/model-capabilities";
import { projectLogicalCapability } from "../src/lib/user-session";
import { capabilitySpecFromChannelModel, mergeCapabilitySpecs } from "../src/pages/admin/logical-models/model-routing-capabilities";
import { workbenchVideoOperation } from "../src/pages/workbench/workbench-input";
import { prepareBackendGenerationTask } from "../src/services/api/generation-task";
import { createModelChannel, defaultConfig, encodeChannelModel } from "../src/stores/use-config-store";
import { generationSpecMetadata, specFromConfig } from "../src/lib/canvas/generation-contract";
import type { ChannelModel } from "../src/services/api/wallet";
import { defaultPriceTier, priceTierPayloadFromForm, skuSelectorFromForm } from "../src/pages/admin/components/channel-model-price-tier-form";
import { modelRequestOptions } from "../src/lib/model-selection";
import { creationVideoConfig } from "../src/pages/create/creation-generation-config";

const schema = manifest.contributes.providers[0].inputSchema as unknown as InputSchema;
const preset = () => structuredClone(schema.templates!.find((template) => template.id === "video-1.5")!.config);

describe("plugin fields and model-owned limits", () => {
    test("a future model can widen presets without another host/model-name branch", () => {
        const configured = preset();
        configured.fields.find((field) => field.name === "videoSeconds")!.max = 30;
        configured.fields.find((field) => field.name === "vquality")!.values!.push("1440p");
        configured.mediaRoles.find((role) => role.role === "reference_image")!.maxCount = 24;
        configured.rules![0].limits![0].values!.push("1440p");
        const inputs = resolveInputConfig(schema, configured);
        const media = Array.from({ length: 24 }, () => ({ kind: "image", role: "reference_image" }));
        const values = modelInputRequest(inputs, { inputParameters: JSON.stringify({ videoSeconds: 30, vquality: "1440p" }) }, media)!;
        expect(values.videoSeconds).toBe(30);
        expect(values.vquality).toBe("1440p");
        expect(() => modelInputRequest(inputs, { inputParameters: JSON.stringify(values) }, [...media, media[0]])).toThrow("24");
        expect(() => modelInputRequest(inputs, { inputParameters: JSON.stringify({ ...values, videoSeconds: 31 }) }, media)).toThrow("范围");
        const original = schema.templates!.find((template) => template.id === "video-1.5")!.config;
        expect(original.fields.find((field) => field.name === "videoSeconds")!.max).toBe(15);
    });

    test("the backend model editor offers real templates and editable roles/rules", () => {
        const value = projectInputConfig(defaultModelCapabilityConfig("xai-video", "grok-future-test"), preset());
        const html = renderToStaticMarkup(<ModelInputConfigEditor schema={schema} model="grok-future-test" section="all" value={value} onChange={() => {}} />);
        expect(html).toContain("应用模板");
        expect(html).toContain('aria-label="参考图最大数量"');
        expect(html).toContain('aria-label="关键帧时间网格"');
        expect(html).toContain("参考音色");
        expect(html).toContain("组合限制");
        const noTemplate = { ...schema, templates: [] };
        expect(renderToStaticMarkup(<ModelInputConfigEditor schema={noTemplate} model="new-model" section="all" value={defaultModelCapabilityConfig()} onChange={() => {}} />)).toContain("自定义能力配置");
        expect(initializeInputConfig(noTemplate, value).fields.find((field) => field.name === "videoSeconds")?.default).toBe(8);
        expect(initializeInputConfig(noTemplate, value).fields.find((field) => field.name === "videoSeconds")).not.toHaveProperty("selection");
    });

    test("generated controls expose voice IDs/keyframe times and conditional 720p", () => {
        const inputs = preset();
        const values = { ...inputDefaults(inputs), vquality: "1080p", reference_audios: [{ voice_id: "eve" }] };
        const html = renderToStaticMarkup(<ModelInputFields config={inputs} values={values} onChange={() => {}} />);
        expect(html).toContain("音色 ID 1");
        expect(html).toContain("最高 720p");
        expect(html).toContain('aria-label="原生音频"');
        expect(() => modelInputRequest(inputs, { inputParameters: JSON.stringify(values) })).toThrow("720p");
        const assignments = renderToStaticMarkup(
            <ModelInputMediaAssignments config={inputs} parameters={values} media={[{ id: "pin", kind: "image" }]} value={JSON.stringify({ pin: { role: "keyframe", inputFields: { timestamp_s: 2 } } })} onChange={() => {}} />,
        );
        expect(assignments).toContain("时间点（秒）");
        expect(workbenchVideoOperation([{ kind: "image", role: "keyframe" }])).toBe("image_to_video");
        expect(effectiveInputFields(inputs, { ...values, vquality: "720p" }, [{ kind: "image", role: "keyframe", inputFields: { timestamp_s: 8 } }]).errors).not.toHaveLength(0);
    });

    test("model config cannot change types, readonly values or binding destinations", () => {
        const input = preset();
        const field = input.fields.find((field) => field.name === "videoSeconds")!;
        field.binding = "providerOptions.other.credential";
        field.type = "string";
        input.fields.find((field) => field.name === "videoGenerateAudio")!.default = false;
        const resolved = resolveInputConfig(schema, input);
        expect(resolved.fields.find((field) => field.name === "videoSeconds")?.binding).toBe("duration");
        expect(resolved.fields.find((field) => field.name === "videoSeconds")?.type).toBe("integer");
        expect(resolved.fields.find((field) => field.name === "videoGenerateAudio")?.default).toBe(true);
    });

    test("submitted task, canvas metadata and public logical catalog retain the same inputs", async () => {
        const inputs = preset();
        const capability = projectInputConfig(defaultModelCapabilityConfig("xai-video", "grok-imagine-video-1.5"), inputs);
        const channel = createModelChannel({ id: "qa-inputs", models: ["grok-imagine-video-1.5"], modelCosts: [{ model: "grok-imagine-video-1.5", capability: "video", protocol: "xai-video", capabilityConfig: capability }] });
        const model = encodeChannelModel(channel.id, "grok-imagine-video-1.5");
        const values = { ...inputDefaults(inputs), reference_audios: [{ voice_id: "eve" }] };
        const config = { ...defaultConfig, channels: [channel], model, videoModel: model, ...inputHostPatch(inputs, values), inputMediaRoles: JSON.stringify({ pin: { role: "keyframe", inputFields: { timestamp_s: 2 } } }) };
        const task = await prepareBackendGenerationTask({ mode: "video", config, prompt: "a scene", referenceImages: [{ id: "pin", name: "pin", type: "image/png", dataUrl: "", url: "https://media.example/pin.png" }] });
        expect((task.input!.config as any).inputParameters.reference_audios).toEqual([{ voice_id: "eve" }]);
        expect((task.input!.referenceImages as any[])[0]).toMatchObject({ id: "pin", role: "keyframe", inputFields: { timestamp_s: 2 } });
        expect(modelRequestOptions(config, "video")).toEqual({ videoSeconds: 8, vquality: "720p" });
        const selected = creationVideoConfig(config, model, { ratio: "9:16", seconds: "12", videoQuality: "720p" });
        expect(selected.videoSeconds).toBe("12");
        expect(modelInputRequest(inputs, selected)?.videoSeconds).toBe(12);
        expect(modelInputRequest(inputs, selected)?.reference_audios).toEqual([{ voice_id: "eve" }]);
        const price = { ...defaultPriceTier(), matchMode: "advanced" as const, imageSelector: { videoSeconds: "12", vquality: "1080p", reference_audios: "eve" } };
        expect(skuSelectorFromForm("video", price, capability)).toEqual({ videoSeconds: "12", vquality: "1080p" });
        expect(priceTierPayloadFromForm("video", price, "grok-imagine-video-1.5", capability).selector).toEqual({ videoSeconds: "12", vquality: "1080p" });
        const spec = specFromConfig("video", "a scene", config);
        expect(generationSpecMetadata(spec).inputParameters).toBe(config.inputParameters);
        expect(generationSpecMetadata(spec).inputMediaRoles).toBe(config.inputMediaRoles);
        const routed = capabilitySpecFromChannelModel({ capability: "video", capabilityConfig: capability } as ChannelModel)!;
        expect(projectLogicalCapability(routed, { videoSeconds: 12 }).inputConfig?.fields.find((field) => field.name === "videoSeconds")?.default).toBe(12);
        expect(mergeCapabilitySpecs("video", [routed]).inputConfig).toEqual(inputs);
        expect(mergeCapabilitySpecs("video", [routed, { ...routed, inputConfig: undefined }]).inputConfig).toBeUndefined();
    });
});
