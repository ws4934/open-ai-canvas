// Protocol shape is separate from editable model presets. No model-name branch
// is needed when another model uses these same fields and media roles.
const operations = ["text_to_video", "image_to_video", "reference_to_video"];
const fields = [
    { name: "videoSeconds", label: "视频时长（秒）", type: "integer", binding: "duration", pricing: true, operations: [...operations, "extend"] },
    { name: "vquality", label: "清晰度", type: "string", binding: "resolution", pricing: true, operations },
    { name: "size", label: "画面比例", type: "string", binding: "aspectRatio", operations },
    { name: "videoGenerateAudio", label: "原生音频", type: "boolean", binding: "generateAudio", readOnly: true, default: true },
    { name: "operation", label: "生成方式", type: "string", binding: "operation", hidden: true },
    { name: "reference_audios", label: "参考音色", type: "object[]", binding: "providerOptions.xai-video.reference_audios", description: "填写官方预设 voice_id；在提示词中用 <AUDIO_0> 等引用。", fields: [{ name: "voice_id", label: "音色 ID", type: "string", required: true, maxLength: 64 }] },
];
const roles = [
    { role: "first_frame", label: "首帧", kind: "image", single: true, maxCount: 0 },
    { role: "last_frame", label: "尾帧", kind: "image", single: true, maxCount: 0 },
    { role: "reference_image", label: "参考图", kind: "image", maxCount: 0 },
    { role: "keyframe", label: "关键帧", kind: "image", maxCount: 0, fields: [{ name: "timestamp_s", label: "时间点（秒）", type: "number", required: true, min: 0, minExclusive: true, upperBoundField: "videoSeconds" }], uniqueField: "timestamp_s", grid: 1 / 3, collectionBinding: "providerOptions.xai-video.keyframes" },
    { role: "reference_video", label: "源视频", kind: "video", single: true, maxCount: 0 },
];
const advanced = { match: "any", when: [{ mediaRole: "reference_image" }, { mediaRole: "last_frame" }, { mediaRole: "keyframe" }, { parameter: "reference_audios" }], limits: [{ name: "vquality", values: ["480p", "720p"] }], message: "此模型的参考、音色或定帧模式最高 720p，请调整清晰度" };
function config(kind) {
    const full = kind === "video-1.5";
    const lite = kind === "video-1.5-lite";
    return {
        operations: lite ? operations.slice(0, 2) : full ? operations : [...operations, "extend", "edit", "video_to_video", "inpaint", "replace_element", "camera_motion", "style_transfer"],
        fields: fields.map(field => ({ ...field,
            ...(field.name === "videoSeconds" ? { min: 1, max: 15, step: 1, default: 8 } : {}),
            ...(field.name === "vquality" ? { values: kind === "video" ? ["480p", "720p"] : ["480p", "720p", "1080p"], default: "720p" } : {}),
            ...(field.name === "size" ? { values: ["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "21:9", "5:2"], default: "16:9" } : {}),
            ...(field.name === "reference_audios" ? { enabled: full, maxItems: 3, default: [] } : {}),
        })),
        mediaRoles: roles.map(role => ({ ...role, ...(role.role === "reference_video" && kind === "video" ? { maxSeconds: 8.7 } : {}), maxCount: role.role === "first_frame" ? 1 : role.role === "reference_image" ? full ? 14 : lite ? 0 : 7 : role.role === "last_frame" ? full ? 1 : 0 : role.role === "keyframe" ? full ? 4 : 0 : kind === "video" ? 1 : 0 })),
        rules: full ? [advanced] : lite ? [] : [
            { when: [{ mediaRole: "first_frame" }], forbidRoles: ["reference_image"], message: "此模型不能混用首帧和参考图" },
            { when: [{ mediaRole: "reference_image" }], limits: [{ name: "videoSeconds", max: 10 }], message: "此模型的参考图模式最多 10 秒" },
            { when: [{ parameter: "operation", values: ["extend"] }], limits: [{ name: "videoSeconds", min: 2, max: 10, default: 6 }], mediaLimits: [{ role: "reference_video", minSeconds: 2, maxSeconds: 15 }], message: "此模型的视频延长新增时长为 2–10 秒" },
            { when: [{ parameter: "operation", values: ["inpaint", "edit", "video_to_video", "replace_element", "camera_motion", "style_transfer"] }], mediaLimits: [{ role: "reference_video", maxSeconds: 8.7 }], message: "此模型的编辑源视频最多 8.7 秒" },
        ],
    };
}
export const xaiVideoInputSchema = {
    fields,
    mediaRoles: roles,
    templates: [
        { id: "video-1.5", label: "Video 1.5：生成与定帧", models: ["grok-imagine-video-1.5"], config: config("video-1.5") },
        { id: "video", label: "Video：生成、编辑与延长", models: ["grok-imagine-video"], config: config("video") },
        { id: "video-1.5-lite", label: "Video 1.5 Lite：基础生成", models: ["grok-imagine-video-1.5-lite"], config: config("video-1.5-lite") },
    ],
};
