import { modelCapabilityConfigFor, normalizeVideoValue } from "@/lib/model-capabilities";
import { resolveModelVideoBooleanOptions } from "@/lib/model-selection";
import type { AiConfig } from "@/stores/use-config-store";
import type { CreationSettings } from "./creation-types";
import { inputHostPatch, inputValuesFor } from "@/lib/model-inputs";

// 创作台没有音频和水印开关；报价与提交共用所选模型的默认值和能力限制。
export function creationVideoConfig(config: AiConfig, model: string, settings: Pick<CreationSettings, "ratio" | "seconds" | "videoQuality">): AiConfig {
    const capability = modelCapabilityConfigFor(config, model);
    const profile = capability.video!;
    const normalized = normalizeVideoValue(profile, {
        ratio: settings.ratio,
        seconds: settings.seconds,
        resolution: settings.videoQuality,
    });
    const result = {
        ...config,
        model,
        videoModel: model,
        size: normalized.ratio,
        videoSeconds: normalized.seconds,
        vquality: normalized.resolution.replace(/p$/i, ""),
        ...resolveModelVideoBooleanOptions(
            config,
            model,
            {},
            {
                videoGenerateAudio: config.videoGenerateAudio,
                videoWatermark: config.videoWatermark,
            },
        ),
    };
    if (capability.inputConfig) {
        const inputs = capability.inputConfig;
        const values = inputValuesFor(inputs, result);
        // This page owns the selected core settings. Persisted extensions may
        // add voices etc., but must not override the duration used for quoting.
        const selected: Record<string, string | number | boolean> = {
            duration: Number(normalized.seconds),
            resolution: normalized.resolution,
            aspectRatio: normalized.ratio,
            generateAudio: result.videoGenerateAudio === "true",
            watermark: result.videoWatermark === "true",
        };
        for (const field of inputs.fields) if (field.enabled !== false && selected[field.binding || ""] !== undefined) values[field.name] = field.readOnly ? field.default! : selected[field.binding!];
        Object.assign(result, inputHostPatch(inputs, values));
    }
    return result;
}
