import type { ImageParameter, ImageParameterValue } from "./image-parameters";
import { imageParameterAccepts } from "./image-parameters";
import type { ModelCapabilityConfig } from "./model-capabilities";

export type InputValue = ImageParameterValue | string[] | Record<string, ImageParameterValue>[];
export type InputField = Omit<ImageParameter, "type" | "default"> & {
    type: ImageParameter["type"] | "string[]" | "object[]";
    default?: InputValue;
    fields?: InputField[];
    maxItems?: number;
    readOnly?: boolean;
    hidden?: boolean;
    minExclusive?: boolean;
    upperBoundField?: string;
    operations?: string[];
};
export type InputRole = {
    role: string;
    label: string;
    kind: "image" | "video" | "audio";
    single?: boolean;
    maxCount: number;
    minSeconds?: number;
    maxSeconds?: number;
    fields?: InputField[];
    uniqueField?: string;
    grid?: number;
    collectionBinding?: string;
};
export type InputCondition = { parameter?: string; mediaRole?: string; values?: ImageParameterValue[] };
export type InputRule = { message: string; match?: "all" | "any"; when: InputCondition[]; limits?: Partial<InputField>[]; forbidRoles?: string[]; mediaLimits?: Pick<InputRole, "role" | "minSeconds" | "maxSeconds">[] };
export type ModelInputConfig = { fields: InputField[]; mediaRoles: InputRole[]; rules?: InputRule[]; operations?: string[] };
export type InputSchema = { fields: InputField[]; mediaRoles: InputRole[]; templates?: { id: string; label: string; models?: string[]; config: ModelInputConfig }[] };
export type InputMedia = { id?: string; label?: string; kind: string; role?: string; durationMs?: number; inputFields?: Record<string, ImageParameterValue> };

export function inputFieldAccepts(field: InputField, value: unknown): value is InputValue {
    if (value === undefined || value === null || value === "") return !field.required;
    if (field.type === "string[]" || field.type === "object[]") {
        if (!Array.isArray(value) || (field.maxItems && value.length > field.maxItems)) return false;
        return value.every((item) =>
            field.type === "string[]"
                ? typeof item === "string" && item.trim() !== "" && (!field.values?.length || field.values.includes(item))
                : item &&
                  typeof item === "object" &&
                  !Array.isArray(item) &&
                  (field.fields || []).every((property) => inputFieldAccepts(property, item[property.name])) &&
                  Object.keys(item).every((name) => field.fields?.some((property) => property.name === name)),
        );
    }
    if (field.minExclusive && typeof value === "number" && field.min !== undefined && value <= field.min) return false;
    return imageParameterAccepts(field as ImageParameter, value);
}

export function parseInputParameters(value: unknown): Record<string, InputValue> {
    if (typeof value === "string") {
        try {
            value = JSON.parse(value);
        } catch {
            return {};
        }
    }
    return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, InputValue>) : {};
}

export function inputDefaults(config: ModelInputConfig) {
    return Object.fromEntries(config.fields.filter((field) => field.enabled !== false && field.default !== undefined).map((field) => [field.name, structuredClone(field.default)])) as Record<string, InputValue>;
}

// Binding/type belongs to the plugin, while constraints belong to this model.
export function resolveInputConfig(schema: InputSchema, config: ModelInputConfig): ModelInputConfig {
    return {
        ...config,
        fields: schema.fields.map((definition) => {
            const setting = config.fields.find((field) => field.name === definition.name);
            if (!setting) return { ...definition };
            return { ...definition, values: setting.values, min: setting.min, max: setting.max, step: setting.step, default: definition.readOnly ? definition.default : setting.default, enabled: setting.enabled, maxItems: setting.maxItems };
        }),
        mediaRoles: schema.mediaRoles.map((definition) => {
            const setting = config.mediaRoles.find((role) => role.role === definition.role);
            return setting ? { ...definition, maxCount: setting.maxCount, grid: setting.grid, minSeconds: setting.minSeconds, maxSeconds: setting.maxSeconds } : { ...definition };
        }),
    };
}

export function initializeInputConfig(schema: InputSchema, value: ModelCapabilityConfig): ModelInputConfig {
    const video = value.video;
    return {
        operations: video?.operations,
        mediaRoles: structuredClone(schema.mediaRoles),
        fields: structuredClone(schema.fields).map((field) => {
            if (!video) return field;
            if (field.binding === "duration")
                return { ...field, min: video.duration.min, max: video.duration.max, step: video.duration.step, default: video.duration.default, values: video.duration.selection === "enum" ? video.duration.values : undefined };
            if (field.binding === "resolution") return { ...field, values: video.resolutions, default: video.defaultResolution };
            if (field.binding === "aspectRatio") return { ...field, values: video.ratios, default: video.defaultRatio };
            return field;
        }),
        rules: [],
    };
}

const hostBindings: Record<string, string> = { duration: "videoSeconds", resolution: "vquality", aspectRatio: "size", generateAudio: "videoGenerateAudio", watermark: "videoWatermark", quality: "quality", imageCount: "count" };

export function inputValuesFor(config: ModelInputConfig, host: { inputParameters?: string; [key: string]: unknown }) {
    const explicit = parseInputParameters(host.inputParameters);
    const values = inputDefaults(config);
    for (const field of config.fields) {
        const raw = explicit[field.name] ?? host[hostBindings[field.binding || ""]];
        if (field.readOnly || field.enabled === false || raw === undefined || raw === null || raw === "") continue;
        values[field.name] = field.type === "number" || field.type === "integer" ? Number(raw) : field.type === "boolean" ? raw === true || raw === "true" : (raw as InputValue);
        if (field.binding === "resolution" && typeof raw === "string" && field.values?.length) {
            const alias = field.values.find((value) => String(value).replace(/p$/i, "").toLowerCase() === raw.replace(/p$/i, "").toLowerCase());
            if (alias !== undefined) values[field.name] = alias;
        }
    }
    return values;
}

export function inputHostPatch(config: ModelInputConfig, values: Record<string, InputValue>) {
    const patch: Record<string, string> = { inputParameters: JSON.stringify(values) };
    for (const field of config.fields) {
        const key = hostBindings[field.binding || ""];
        if (key && values[field.name] !== undefined) patch[key] = String(values[field.name]);
    }
    return patch;
}

function present(value: unknown) {
    return value !== undefined && value !== null && value !== "" && (!Array.isArray(value) || value.length > 0);
}

export function effectiveInputFields(config: ModelInputConfig, values: Record<string, InputValue>, media: InputMedia[] = []) {
    const roleOf = (item: InputMedia) => (item.role === "reference" ? `reference_${item.kind}` : item.role || "");
    let hasFirst = media.some((item) => item.role === "first_frame");
    const unmarked = media.filter((item) => item.kind === "image" && !item.role).length;
    media = media.map((item) => {
        let role = roleOf(item);
        if (!role) {
            role = `reference_${item.kind}`;
            if (item.kind === "image" && unmarked === 1 && !hasFirst) {
                role = "first_frame";
                hasFirst = true;
            }
        }
        return { ...item, role };
    });
    const counts: Record<string, number> = {};
    for (const item of media) counts[item.role!] = (counts[item.role!] || 0) + 1;
    const operationField = config.fields.find((field) => field.binding === "operation");
    const operation = String(
        (operationField && values[operationField.name]) || (media.some((item) => item.kind === "video") ? "edit" : counts.reference_image ? "reference_to_video" : media.some((item) => item.kind === "image") ? "image_to_video" : "text_to_video"),
    );
    values = { ...values, ...(operationField ? { [operationField.name]: operation } : {}) };
    const fields = config.fields.filter((field) => !operation || !field.operations?.length || field.operations.includes(operation)).map((field) => ({ ...field }));
    const roles = config.mediaRoles.map((role) => ({ ...role }));
    const errors: string[] = [];
    for (const item of media) if (!config.mediaRoles.some((role) => role.role === item.role && role.kind === item.kind)) errors.push(`当前模型不支持素材角色 ${item.role}`);
    if (operation && config.operations?.length && !config.operations.includes(operation)) errors.push(`当前模型不支持操作 ${operation}`);
    for (const rule of config.rules || []) {
        const matches = rule.when.map((condition) =>
            condition.mediaRole ? (counts[condition.mediaRole] || 0) > 0 : condition.values?.length ? condition.values.includes(values[condition.parameter!] as ImageParameterValue) : present(values[condition.parameter!]),
        );
        if (!(rule.match === "any" ? matches.some(Boolean) : matches.every(Boolean))) continue;
        if (rule.forbidRoles?.some((role) => counts[role])) errors.push(rule.message);
        for (const limit of rule.mediaLimits || []) {
            const role = roles.find((role) => role.role === limit.role);
            if (role) {
                role.minSeconds = limit.minSeconds;
                role.maxSeconds = limit.maxSeconds;
            }
        }
        for (const limit of rule.limits || []) {
            const index = fields.findIndex((field) => field.name === limit.name);
            if (index < 0) continue;
            fields[index] = { ...fields[index], ...limit } as InputField;
            if (present(values[limit.name!]) && !inputFieldAccepts(fields[index], values[limit.name!])) errors.push(rule.message);
        }
    }
    for (const field of fields) {
        if (field.enabled !== false && !inputFieldAccepts(field, values[field.name])) errors.push(`${field.label}超出当前模型支持范围`);
    }
    for (const role of roles) {
        if ((counts[role.role] || 0) > role.maxCount) errors.push(`${role.label}最多允许 ${role.maxCount} 项`);
        const slots = new Set<number>();
        for (const item of media.filter((item) => item.role === role.role)) {
            const seconds = (item.durationMs || 0) / 1000;
            if (seconds > 0 && (seconds < (role.minSeconds || 0) || (role.maxSeconds && seconds > role.maxSeconds))) errors.push(`${role.label}时长超出当前模型支持范围`);
            for (const field of role.fields || []) {
                const value = item.inputFields?.[field.name];
                if (!inputFieldAccepts(field, value) || (field.upperBoundField && Number(value) >= Number(values[field.upperBoundField]))) errors.push(`${role.label}：${field.label}无效，需小于视频时长`);
            }
            if (role.uniqueField && role.grid) {
                const slot = Math.round(Number(item.inputFields?.[role.uniqueField]) / role.grid);
                if (slots.has(slot)) errors.push(`${role.label}不能落在同一个时间网格`);
                slots.add(slot);
            }
        }
    }
    return { fields, errors: Array.from(new Set(errors)) };
}

export function modelInputRequest(config: ModelInputConfig | undefined, host: { inputParameters?: string; [key: string]: unknown }, media: InputMedia[] = []) {
    if (!config) return undefined;
    const values = inputValuesFor(config, host);
    const { errors } = effectiveInputFields(config, values, media);
    if (errors.length) throw new Error(errors[0]);
    const { fields } = effectiveInputFields(config, values, media);
    return Object.fromEntries(fields.filter((field) => field.enabled !== false && !field.hidden && values[field.name] !== undefined).map((field) => [field.name, values[field.name]]));
}

export function projectInputConfig(value: ModelCapabilityConfig, config: ModelInputConfig): ModelCapabilityConfig {
    if (!value.video) return { ...value, inputConfig: config };
    const video = { ...value.video, references: { ...value.video.references }, duration: { ...value.video.duration } };
    for (const field of config.fields) {
        if (field.binding === "duration")
            video.duration = {
                selection: field.values?.length ? "enum" : "range",
                values: field.values?.map(Number),
                min: field.min ?? video.duration.min,
                max: field.max ?? video.duration.max,
                step: field.step ?? video.duration.step,
                default: Number(field.default ?? video.duration.default),
            };
        if (field.binding === "resolution") {
            video.resolutions = field.values?.map(String) || [];
            video.defaultResolution = String(field.default ?? video.defaultResolution);
        }
        if (field.binding === "aspectRatio") {
            video.ratios = field.values?.map(String) || [];
            video.defaultRatio = String(field.default ?? video.defaultRatio);
        }
        if (field.binding === "generateAudio") video.generateAudio = { supported: field.enabled !== false, default: field.default === true, fixed: field.readOnly };
    }
    if (config.operations?.length) {
        video.operations = [...config.operations];
        video.defaultOperation = config.operations[0];
    }
    video.references.maxImages = config.mediaRoles.filter((role) => role.kind === "image").reduce((sum, role) => sum + role.maxCount, 0);
    video.references.maxVideos = config.mediaRoles.filter((role) => role.kind === "video").reduce((sum, role) => sum + role.maxCount, 0);
    video.references.maxAudios = config.mediaRoles.filter((role) => role.kind === "audio").reduce((sum, role) => sum + role.maxCount, 0);
    video.references.maxVideoDurationSeconds = 0;
    video.references.maxAudioDurationSeconds = 0;
    return { ...value, inputConfig: config, video };
}

export function inputMediaAssignments(value?: string): Record<string, { role: string; inputFields?: Record<string, ImageParameterValue> }> {
    if (!value) return {};
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("素材角色配置无效");
    return parsed;
}
