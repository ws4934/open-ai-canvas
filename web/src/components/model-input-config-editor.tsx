import { Button, Input, InputNumber, Select, Switch } from "antd";
import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { InputConstraintEditor } from "./model-input-fields";
import { formatInputGrid, initializeInputConfig, parseInputGrid, projectInputConfig, resolveInputConfig, type InputField, type InputRole, type InputRule, type InputSchema, type ModelInputConfig } from "@/lib/model-inputs";
import type { ModelCapabilityConfig } from "@/lib/model-capabilities";
import "./model-input-config-editor.css";

type Props = { schema: InputSchema; value: ModelCapabilityConfig; model: string; section: "all" | "references" | "protocol"; disabled?: boolean; onChange: (value: ModelCapabilityConfig) => void };

const operationLabels: Record<string, string> = {
    text_to_video: "文生视频",
    image_to_video: "图生视频",
    reference_to_video: "参考图生视频",
    video_to_video: "视频编辑",
    edit: "编辑",
    extend: "视频延长",
    inpaint: "局部修改",
    replace_element: "元素替换",
    camera_motion: "运镜调整",
    style_transfer: "风格迁移",
    audio_to_video: "音频生视频",
    enhance: "画质增强",
};

export function ModelInputConfigEditor({ schema, value, model, section, disabled, onChange }: Props) {
    const [selectedTemplate, setSelectedTemplate] = useState<string>();
    const [roleCounts, setRoleCounts] = useState<Record<string, number>>({});
    const templateID = schema.templates?.find((template) => template.id === selectedTemplate)?.id || schema.templates?.find((template) => template.models?.includes(model))?.id;
    const config = value.inputConfig;
    const update = (inputConfig: ModelInputConfig) => onChange(projectInputConfig(value, resolveInputConfig(schema, inputConfig)));
    const applyTemplate = () => {
        const template = schema.templates?.find((template) => template.id === templateID);
        if (template) {
            setRoleCounts({});
            update(structuredClone(template.config));
        }
    };
    const templatePicker = schema.templates?.length ? (
        <div className="model-input-template-controls">
            <Select aria-label="模型能力模板" disabled={disabled} value={templateID} placeholder="选择能力模板" options={schema.templates.map((template) => ({ value: template.id, label: template.label }))} onChange={setSelectedTemplate} />
            <Button disabled={disabled || !templateID} onClick={applyTemplate}>
                应用模板
            </Button>
        </div>
    ) : null;
    if (!config)
        return (
            <div className="model-input-config-editor">
                <section className="model-input-panel model-input-initialize">
                    <div className="model-input-section-heading">
                        <div>
                            <h4>设置模型能力</h4>
                            <p>选择模板，或从当前配置开始调整。价格保持不变。</p>
                        </div>
                        <Button disabled={disabled} onClick={() => update(initializeInputConfig(schema, value))}>
                            自定义能力配置
                        </Button>
                    </div>
                    {templatePicker}
                </section>
            </div>
        );
    const activeRoles = config.mediaRoles.filter((role) => role.maxCount > 0);
    const inactiveRoles = config.mediaRoles.filter((role) => role.maxCount === 0);
    const changeRole = (role: InputRole, patch: Partial<InputRole>) => {
        if (patch.maxCount === 0 && role.maxCount > 0) setRoleCounts((counts) => ({ ...counts, [role.role]: role.maxCount }));
        update({ ...config, mediaRoles: config.mediaRoles.map((item) => (item.role === role.role ? { ...item, ...patch } : item)) });
    };
    const updateReferences = (patch: Partial<NonNullable<ModelCapabilityConfig["video"]>["references"]>) => {
        if (value.video) onChange({ ...value, video: { ...value.video, references: { ...value.video.references, ...patch } } });
    };
    return (
        <div className="model-input-config-editor">
            {section !== "protocol" ? (
                <div className="model-input-reference-layout">
                    <section className="model-input-panel">
                        <h4 className="model-input-panel-heading">输入素材</h4>
                        {activeRoles.length ? (
                            <div className="model-input-role-list">
                                {activeRoles.map((role) => (
                                    <ModelInputRoleEditor key={role.role} role={role} disabled={disabled} onChange={(patch) => changeRole(role, patch)} />
                                ))}
                            </div>
                        ) : (
                            <p className="model-input-empty">当前未开放素材输入</p>
                        )}
                        {inactiveRoles.length ? (
                            <details className="model-input-inactive-roles">
                                <summary>未启用的素材（{inactiveRoles.length}）</summary>
                                <div className="model-input-role-options">
                                    {inactiveRoles.map((role) => (
                                        <Button key={role.role} size="small" icon={<Plus size={14} />} aria-label={`启用${role.label}`} disabled={disabled} onClick={() => changeRole(role, { maxCount: roleCounts[role.role] || 1 })}>
                                            {role.label}
                                        </Button>
                                    ))}
                                </div>
                            </details>
                        ) : null}
                    </section>
                    {value.video ? (
                        <section className="model-input-panel">
                            <h4 className="model-input-panel-heading">请求限制</h4>
                            <div className="model-input-limits-grid">
                                <label className="model-input-label">
                                    <span>提示词最大字符数</span>
                                    <InputNumber aria-label="提示词最大字符数" min={1} disabled={disabled} value={value.video.references.promptMaxChars} onChange={(number) => updateReferences({ promptMaxChars: number || 1 })} />
                                </label>
                                {(["image", "video", "audio"] as const)
                                    .filter((kind) => activeRoles.some((role) => role.kind === kind))
                                    .map((kind) => {
                                        const key = kind === "image" ? "maxImageBytes" : kind === "video" ? "maxVideoBytes" : "maxAudioBytes";
                                        const label = kind === "image" ? "单张图片上限 MB" : kind === "video" ? "单个视频上限 MB" : "单个音频上限 MB";
                                        return (
                                            <label key={kind} className="model-input-label">
                                                <span>{label}</span>
                                                <InputNumber aria-label={label} min={0} disabled={disabled} value={value.video!.references[key] / 1024 / 1024} onChange={(number) => updateReferences({ [key]: (number || 0) * 1024 * 1024 })} />
                                            </label>
                                        );
                                    })}
                            </div>
                        </section>
                    ) : null}
                </div>
            ) : null}
            {section !== "references" ? (
                <>
                    {templatePicker ? (
                        <div className="model-input-template-bar">
                            <div>
                                <span className="model-input-template-title">能力模板</span>
                                <p>应用后替换参数、素材和组合限制，价格保持不变。</p>
                            </div>
                            {templatePicker}
                        </div>
                    ) : null}
                    <div className="model-input-parameter-grid">
                        {config.fields
                            .filter((field) => !field.hidden)
                            .map((field) => (
                                <section key={field.name} className="model-input-panel model-input-parameter" data-enabled={field.enabled !== false}>
                                    <div className="model-input-parameter-heading">
                                        <div>
                                            <h4>{field.label}</h4>
                                            {field.pricing ? <span className="model-input-meta">可计价</span> : null}
                                        </div>
                                        {field.readOnly ? (
                                            <span className="model-input-meta">固定</span>
                                        ) : (
                                            <Switch
                                                aria-label={`开放${field.label}`}
                                                disabled={disabled || field.required}
                                                checked={field.enabled !== false}
                                                onChange={(enabled) => update({ ...config, fields: config.fields.map((item) => (item.name === field.name ? { ...item, enabled } : item)) })}
                                            />
                                        )}
                                    </div>
                                    {field.enabled !== false ? <InputConstraintEditor field={field} disabled={disabled} onChange={(next) => update({ ...config, fields: config.fields.map((item) => (item.name === field.name ? next : item)) })} /> : null}
                                </section>
                            ))}
                    </div>
                    <section className="model-input-panel">
                        <label className="model-input-label">
                            <span>支持的生成方式</span>
                            <Select
                                aria-label="支持的生成方式"
                                mode="tags"
                                disabled={disabled}
                                value={config.operations || []}
                                onChange={(operations) => update({ ...config, operations })}
                                options={Array.from(new Set([...(config.operations || []), ...(schema.templates?.flatMap((template) => template.config.operations || []) || [])])).map((value) => ({ value, label: operationLabels[value] || value }))}
                            />
                        </label>
                    </section>
                    <details className="model-input-panel model-input-rules">
                        <summary>
                            组合限制<span className="model-input-meta">{config.rules?.length || 0} 条</span>
                        </summary>
                        <ModelInputRulesEditor config={config} disabled={disabled} onChange={(rules) => update({ ...config, rules })} />
                    </details>
                </>
            ) : null}
        </div>
    );
}

function ModelInputRoleEditor({ role, disabled, onChange }: { role: InputRole; disabled?: boolean; onChange: (patch: Partial<InputRole>) => void }) {
    return (
        <div className="model-input-role">
            <div className="model-input-role-heading">
                <span className="model-input-role-name">{role.label}</span>
                {role.single ? (
                    <span className="model-input-single">最多 1 {role.kind === "image" ? "张" : "个"}</span>
                ) : (
                    <InputNumber aria-label={`${role.label}最大数量`} min={1} precision={0} disabled={disabled} value={role.maxCount} prefix="最多" suffix={role.kind === "image" ? "张" : "个"} onChange={(count) => onChange({ maxCount: count || 1 })} />
                )}
                <Switch aria-label={`开放${role.label}`} disabled={disabled} checked onChange={() => onChange({ maxCount: 0 })} />
            </div>
            {role.kind !== "image" || role.uniqueField ? (
                <div className="model-input-role-details">
                    {role.kind !== "image"
                        ? (
                              [
                                  ["minSeconds", "最短秒数"],
                                  ["maxSeconds", "最长秒数"],
                              ] as const
                          ).map(([name, label]) => (
                              <label key={name} className="model-input-label">
                                  <span>{label}</span>
                                  <InputNumber aria-label={`${role.label}${label}`} disabled={disabled} min={0} value={role[name]} placeholder="不限" onChange={(value) => onChange({ [name]: value ?? undefined })} />
                              </label>
                          ))
                        : null}
                    {role.uniqueField ? (
                        <label className="model-input-label">
                            <span>时间网格（秒）</span>
                            <InputNumber
                                aria-label={`${role.label}时间网格`}
                                disabled={disabled}
                                min={0}
                                value={role.grid}
                                formatter={(number, info) => (info.userTyping ? info.input : formatInputGrid(number === undefined ? undefined : Number(number)))}
                                parser={(text) => parseInputGrid(text) ?? NaN}
                                placeholder="不分格"
                                onChange={(value) => onChange({ grid: value ?? undefined })}
                            />
                        </label>
                    ) : null}
                </div>
            ) : null}
        </div>
    );
}

function ModelInputRulesEditor({ config, onChange, disabled }: { config: ModelInputConfig; onChange: (rules: InputRule[]) => void; disabled?: boolean }) {
    const rules = config.rules || [];
    const update = (index: number, patch: Partial<InputRule>) => onChange(rules.map((rule, position) => (position === index ? { ...rule, ...patch } : rule)));
    const conditions = [...config.mediaRoles.map((role) => ({ label: `含${role.label}`, value: `media:${role.role}` })), ...config.fields.map((field) => ({ label: `${field.label}已填写 / 指定值`, value: `field:${field.name}` }))];
    return (
        <section className="space-y-3">
            <div className="flex justify-end">
                <Button disabled={disabled} size="small" icon={<Plus size={14} />} onClick={() => onChange([...rules, { message: "请调整当前参数组合", when: [{ mediaRole: config.mediaRoles[0]?.role }], limits: [] }])}>
                    添加规则
                </Button>
            </div>
            {rules.map((rule, index) => (
                <div key={index} className="space-y-3 rounded-lg border border-border p-4">
                    <div className="flex gap-2">
                        <Input aria-label={`规则 ${index + 1} 提示`} disabled={disabled} value={rule.message} onChange={(event) => update(index, { message: event.target.value })} />
                        <Button type="text" disabled={disabled} aria-label={`删除规则 ${index + 1}`} icon={<Trash2 size={15} />} onClick={() => onChange(rules.filter((_, position) => position !== index))} />
                    </div>
                    <Select
                        aria-label={`规则 ${index + 1} 匹配方式`}
                        disabled={disabled}
                        value={rule.match || "all"}
                        options={[
                            { value: "all", label: "满足全部条件" },
                            { value: "any", label: "满足任一条件" },
                        ]}
                        onChange={(match) => update(index, { match })}
                    />
                    {rule.when.map((condition, conditionIndex) => (
                        <div key={conditionIndex} className="flex flex-wrap gap-2">
                            <Select
                                aria-label={`规则 ${index + 1} 条件 ${conditionIndex + 1}`}
                                className="min-w-44 flex-1"
                                disabled={disabled}
                                options={conditions}
                                value={condition.mediaRole ? `media:${condition.mediaRole}` : `field:${condition.parameter}`}
                                onChange={(value) => {
                                    const [kind, name] = value.split(":");
                                    update(index, { when: rule.when.map((item, position) => (position === conditionIndex ? (kind === "media" ? { mediaRole: name } : { parameter: name }) : item)) });
                                }}
                            />
                            {condition.parameter ? (
                                <Select
                                    aria-label="条件支持值"
                                    mode="tags"
                                    className="min-w-32 flex-1"
                                    disabled={disabled}
                                    value={condition.values?.map(String) || []}
                                    placeholder="留空表示已填写"
                                    onChange={(values) => {
                                        const type = config.fields.find((field) => field.name === condition.parameter)?.type;
                                        update(index, {
                                            when: rule.when.map((item, position) =>
                                                position === conditionIndex ? { ...item, values: values.map((value) => (type === "integer" || type === "number" ? Number(value) : type === "boolean" ? value === "true" : value)) } : item,
                                            ),
                                        });
                                    }}
                                />
                            ) : null}
                            <Button disabled={disabled} aria-label="移除此条件" onClick={() => update(index, { when: rule.when.filter((_, position) => position !== conditionIndex) })}>
                                移除
                            </Button>
                        </div>
                    ))}
                    <Button size="small" disabled={disabled} onClick={() => update(index, { when: [...rule.when, { mediaRole: config.mediaRoles[0]?.role }] })}>
                        添加条件
                    </Button>
                    {(rule.limits || []).map((limit, limitIndex) => {
                        const field = config.fields.find((field) => field.name === limit.name);
                        return (
                            <div key={limitIndex} className="space-y-2 rounded-md bg-surface-secondary p-3">
                                <div className="flex gap-2">
                                    <Select
                                        aria-label="受限参数"
                                        className="min-w-0 flex-1"
                                        disabled={disabled}
                                        value={limit.name}
                                        options={config.fields.filter((field) => !field.hidden).map((field) => ({ value: field.name, label: field.label }))}
                                        onChange={(name) => update(index, { limits: rule.limits?.map((item, position) => (position === limitIndex ? { name } : item)) })}
                                    />
                                    <Button disabled={disabled} onClick={() => update(index, { limits: rule.limits?.filter((_, position) => position !== limitIndex) })}>
                                        移除限制
                                    </Button>
                                </div>
                                {field ? (
                                    <InputConstraintEditor
                                        field={{ ...field, ...limit } as InputField}
                                        disabled={disabled}
                                        onChange={(next) =>
                                            update(index, {
                                                limits: rule.limits?.map((item, position) =>
                                                    position === limitIndex ? { name: next.name, values: next.values, min: next.min, max: next.max, step: next.step, maxItems: next.maxItems, default: next.default } : item,
                                                ),
                                            })
                                        }
                                    />
                                ) : null}
                            </div>
                        );
                    })}
                    <div className="flex flex-wrap items-center gap-2">
                        <Button size="small" disabled={disabled} onClick={() => update(index, { limits: [...(rule.limits || []), { name: config.fields.find((field) => !field.hidden)?.name }] })}>
                            添加参数限制
                        </Button>
                        <Select
                            aria-label="禁止同时使用的素材"
                            mode="multiple"
                            className="min-w-52 flex-1"
                            disabled={disabled}
                            placeholder="禁止同时使用的素材"
                            value={rule.forbidRoles || []}
                            options={config.mediaRoles.map((role) => ({ value: role.role, label: role.label }))}
                            onChange={(forbidRoles) => update(index, { forbidRoles })}
                        />
                    </div>
                    {(rule.mediaLimits || []).map((limit, limitIndex) => (
                        <div key={limitIndex} className="flex flex-wrap gap-2">
                            <Select
                                aria-label="受限素材"
                                disabled={disabled}
                                value={limit.role}
                                options={config.mediaRoles.filter((role) => role.kind !== "image").map((role) => ({ value: role.role, label: role.label }))}
                                onChange={(role) => update(index, { mediaLimits: rule.mediaLimits?.map((item, position) => (position === limitIndex ? { ...item, role } : item)) })}
                            />
                            {(
                                [
                                    ["minSeconds", "最短秒数"],
                                    ["maxSeconds", "最长秒数"],
                                ] as const
                            ).map(([name, label]) => (
                                <InputNumber
                                    key={name}
                                    aria-label={`规则 ${index + 1} 素材${label}`}
                                    disabled={disabled}
                                    min={0}
                                    value={limit[name]}
                                    placeholder={label}
                                    onChange={(value) => update(index, { mediaLimits: rule.mediaLimits?.map((item, position) => (position === limitIndex ? { ...item, [name]: value ?? undefined } : item)) })}
                                />
                            ))}
                            <Button disabled={disabled} onClick={() => update(index, { mediaLimits: rule.mediaLimits?.filter((_, position) => position !== limitIndex) })}>
                                移除时长限制
                            </Button>
                        </div>
                    ))}
                    {config.mediaRoles.some((role) => role.kind !== "image") ? (
                        <Button size="small" disabled={disabled} onClick={() => update(index, { mediaLimits: [...(rule.mediaLimits || []), { role: config.mediaRoles.find((role) => role.kind !== "image")!.role }] })}>
                            添加素材时长限制
                        </Button>
                    ) : null}
                </div>
            ))}
        </section>
    );
}
