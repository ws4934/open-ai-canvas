import { Button, Input, InputNumber, Select, Switch } from "antd";
import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { InputConstraintEditor } from "./model-input-fields";
import { initializeInputConfig, projectInputConfig, resolveInputConfig, type InputField, type InputRule, type InputSchema, type ModelInputConfig } from "@/lib/model-inputs";
import type { ModelCapabilityConfig } from "@/lib/model-capabilities";

type Props = { schema: InputSchema; value: ModelCapabilityConfig; model: string; section: "all" | "references" | "protocol"; disabled?: boolean; onChange: (value: ModelCapabilityConfig) => void };

export function ModelInputConfigEditor({ schema, value, model, section, disabled, onChange }: Props) {
    const [templateID, setTemplateID] = useState(schema.templates?.find((template) => template.models?.includes(model))?.id);
    const config = value.inputConfig;
    const update = (inputConfig: ModelInputConfig) => onChange(projectInputConfig(value, resolveInputConfig(schema, inputConfig)));
    const applyTemplate = () => {
        const template = schema.templates?.find((template) => template.id === templateID);
        if (template) update(structuredClone(template.config));
    };
    if (!config)
        return (
            <div className="space-y-3 rounded-lg border border-border bg-surface-secondary p-4">
                <div className="text-sm font-medium">使用插件参数表单</div>
                <p className="m-0 text-sm text-foreground/65">选择一个能力模板，再按实际模型调整范围。型号不在模板中也可以使用，应用模板不会修改价格。</p>
                {schema.templates?.length ? (
                    <div className="flex gap-2">
                        <Select
                            aria-label="模型能力模板"
                            className="min-w-0 flex-1"
                            disabled={disabled}
                            value={templateID}
                            placeholder="选择接近的能力模板"
                            options={schema.templates.map((template) => ({ value: template.id, label: template.label }))}
                            onChange={setTemplateID}
                        />
                        <Button disabled={disabled || !templateID} onClick={applyTemplate}>
                            应用模板
                        </Button>
                    </div>
                ) : null}
                <Button disabled={disabled} onClick={() => update(initializeInputConfig(schema, value))}>
                    自定义能力配置
                </Button>
            </div>
        );
    return (
        <div className="space-y-4">
            {section !== "protocol" ? (
                <>
                    <div className="grid gap-3 sm:grid-cols-2">
                        {config.mediaRoles.map((role) => {
                            const changeRole = (patch: Partial<typeof role>) => update({ ...config, mediaRoles: config.mediaRoles.map((item) => (item.role === role.role ? { ...item, ...patch } : item)) });
                            return (
                                <section key={role.role} className="space-y-3 rounded-lg border border-border p-3 text-sm">
                                    <label className="flex items-center justify-between gap-3">
                                        <span>
                                            {role.label}
                                            <span className="ml-2 text-xs text-foreground/50">{role.kind}</span>
                                        </span>
                                        <InputNumber aria-label={`${role.label}最大数量`} min={0} max={role.single ? 1 : undefined} disabled={disabled} value={role.maxCount} onChange={(count) => changeRole({ maxCount: count || 0 })} />
                                    </label>
                                    {role.kind !== "image" ? (
                                        <div className="grid grid-cols-2 gap-2">
                                            {(
                                                [
                                                    ["minSeconds", "最短秒数"],
                                                    ["maxSeconds", "最长秒数"],
                                                ] as const
                                            ).map(([name, label]) => (
                                                <label key={name} className="space-y-1 text-xs text-foreground/65">
                                                    {label}
                                                    <InputNumber className="w-full" aria-label={`${role.label}${label}`} disabled={disabled} min={0} value={role[name]} placeholder="不限" onChange={(value) => changeRole({ [name]: value ?? undefined })} />
                                                </label>
                                            ))}
                                        </div>
                                    ) : null}
                                    {role.uniqueField ? (
                                        <label className="block space-y-1 text-xs text-foreground/65">
                                            时间网格（秒，留空不分格）
                                            <InputNumber className="w-full" aria-label={`${role.label}时间网格`} disabled={disabled} min={0} value={role.grid} onChange={(value) => changeRole({ grid: value ?? undefined })} />
                                        </label>
                                    ) : null}
                                    {role.fields?.length ? <p className="m-0 text-xs text-foreground/55">逐项填写：{role.fields.map((field) => field.label).join("、")}</p> : null}
                                </section>
                            );
                        })}
                    </div>
                    {value.video ? (
                        <div className="grid gap-3 sm:grid-cols-2">
                            <label className="space-y-1 text-xs text-foreground/65">
                                提示词最大字符数
                                <InputNumber
                                    aria-label="提示词最大字符数"
                                    className="w-full"
                                    min={1}
                                    disabled={disabled}
                                    value={value.video.references.promptMaxChars}
                                    onChange={(number) => onChange({ ...value, video: { ...value.video!, references: { ...value.video!.references, promptMaxChars: number || 1 } } })}
                                />
                            </label>
                            <label className="space-y-1 text-xs text-foreground/65">
                                单张图片上限 MB
                                <InputNumber
                                    aria-label="单张图片上限 MB"
                                    className="w-full"
                                    min={0}
                                    disabled={disabled}
                                    value={value.video.references.maxImageBytes / 1024 / 1024}
                                    onChange={(number) => onChange({ ...value, video: { ...value.video!, references: { ...value.video!.references, maxImageBytes: (number || 0) * 1024 * 1024 } } })}
                                />
                            </label>
                        </div>
                    ) : null}
                </>
            ) : null}
            {section !== "references" ? (
                <>
                    <div className="flex flex-wrap items-center justify-between gap-3">
                        <span className="text-sm text-foreground/65">字段来自插件；支持值与上限按当前模型设置，不受模板原型号限制。</span>
                        {schema.templates?.length ? (
                            <div className="flex gap-2">
                                <Select aria-label="更换能力模板" disabled={disabled} value={templateID} placeholder="选择模板" options={schema.templates.map((template) => ({ value: template.id, label: template.label }))} onChange={setTemplateID} />
                                <Button disabled={disabled || !templateID} onClick={applyTemplate}>
                                    应用模板
                                </Button>
                            </div>
                        ) : null}
                    </div>
                    {config.fields
                        .filter((field) => !field.hidden)
                        .map((field) => (
                            <section key={field.name} className="space-y-3 rounded-lg border border-border p-4">
                                <div className="flex items-center justify-between gap-3">
                                    <span className="text-sm font-medium">
                                        {field.label}
                                        {field.pricing ? <span className="ml-2 text-xs font-normal text-foreground/50">可计价</span> : null}
                                    </span>
                                    <Switch
                                        aria-label={`开放${field.label}`}
                                        disabled={disabled || field.required || field.readOnly}
                                        checked={field.enabled !== false}
                                        onChange={(enabled) => update({ ...config, fields: config.fields.map((item) => (item.name === field.name ? { ...item, enabled } : item)) })}
                                    />
                                </div>
                                <InputConstraintEditor field={field} disabled={disabled || field.enabled === false} onChange={(next) => update({ ...config, fields: config.fields.map((item) => (item.name === field.name ? next : item)) })} />
                            </section>
                        ))}
                    <label className="block space-y-1 text-xs text-foreground/65">
                        支持的生成方式
                        <Select
                            aria-label="支持的生成方式"
                            mode="tags"
                            className="w-full"
                            disabled={disabled}
                            value={config.operations || []}
                            onChange={(operations) => update({ ...config, operations })}
                            options={Array.from(new Set(schema.templates?.flatMap((template) => template.config.operations || []) || [])).map((value) => ({ value, label: value }))}
                        />
                    </label>
                    <ModelInputRulesEditor config={config} disabled={disabled} onChange={(rules) => update({ ...config, rules })} />
                </>
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
            <div className="flex items-center justify-between">
                <span className="text-sm font-medium">组合限制</span>
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
