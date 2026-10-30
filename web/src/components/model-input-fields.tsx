import { Alert, Button, Input, InputNumber, Select, Switch } from "antd";
import { Plus, Trash2 } from "lucide-react";
import { ImageParameterInput } from "./image-parameter-fields";
import type { ImageParameter, ImageParameterValue } from "@/lib/image-parameters";
import { effectiveInputFields, inputMediaAssignments, type InputField, type InputMedia, type InputRole, type InputValue, type ModelInputConfig } from "@/lib/model-inputs";

export function ModelInputField({ field, value, onChange, disabled }: { field: InputField; value?: InputValue; onChange: (value: InputValue | undefined) => void; disabled?: boolean }) {
    if (field.type === "string[]")
        return (
            <Select
                aria-label={field.label}
                mode="tags"
                className="w-full"
                value={Array.isArray(value) ? (value as string[]) : []}
                options={field.values?.map((value) => ({ value: String(value), label: String(value) }))}
                maxCount={field.maxItems || undefined}
                disabled={disabled || field.readOnly}
                onChange={onChange}
            />
        );
    if (field.type === "object[]") {
        const items = Array.isArray(value) ? (value as Record<string, ImageParameterValue>[]) : [];
        return (
            <div className="space-y-2">
                {items.map((item, index) => (
                    <div key={index} className="flex items-end gap-2">
                        {(field.fields || []).map((property) => (
                            <label key={property.name} className="min-w-0 flex-1 space-y-1 text-xs text-foreground/65">
                                <span>
                                    {property.label} {index + 1}
                                </span>
                                <ModelInputField
                                    field={property}
                                    value={item[property.name]}
                                    disabled={disabled}
                                    onChange={(value) => onChange(items.map((current, position) => (position === index ? { ...current, [property.name]: value as ImageParameterValue } : current)))}
                                />
                            </label>
                        ))}
                        <Button type="text" disabled={disabled} aria-label={`移除${field.label} ${index + 1}`} icon={<Trash2 size={15} />} onClick={() => onChange(items.filter((_, position) => position !== index))} />
                    </div>
                ))}
                <Button
                    size="small"
                    disabled={disabled || Boolean(field.maxItems && items.length >= field.maxItems)}
                    icon={<Plus size={14} />}
                    onClick={() => onChange([...items, Object.fromEntries((field.fields || []).filter((property) => property.default !== undefined).map((property) => [property.name, property.default]))] as Record<string, ImageParameterValue>[])}
                >
                    添加{field.label}
                    {field.maxItems ? `（最多 ${field.maxItems} 项）` : ""}
                </Button>
            </div>
        );
    }
    return <ImageParameterInput field={field as ImageParameter} value={value as ImageParameterValue | undefined} disabled={disabled || field.readOnly} onChange={onChange} />;
}

export function ModelInputFields({ config, values, media = [], onChange, disabled }: { config: ModelInputConfig; values: Record<string, InputValue>; media?: InputMedia[]; onChange: (values: Record<string, InputValue>) => void; disabled?: boolean }) {
    const effective = effectiveInputFields(config, values, media);
    return (
        <div className="space-y-3" data-canvas-no-zoom data-canvas-wheel-scroll>
            {effective.fields
                .filter((field) => field.enabled !== false && !field.hidden)
                .map((field) => (
                    <div key={field.name} className="space-y-1.5">
                        <div className="text-xs font-medium text-foreground/70">
                            {field.label}
                            {field.required ? " *" : ""}
                        </div>
                        <ModelInputField
                            field={field}
                            value={values[field.name]}
                            disabled={disabled}
                            onChange={(value) => {
                                const next = { ...values };
                                if (value === undefined) delete next[field.name];
                                else next[field.name] = value;
                                onChange(next);
                            }}
                        />
                        {field.description ? <p className="m-0 text-xs leading-relaxed text-foreground/55">{field.description}</p> : null}
                    </div>
                ))}
            {effective.errors.length ? <Alert type="warning" showIcon title={effective.errors[0]} /> : null}
        </div>
    );
}

export function ModelMediaFields({ role, values, parameters, onChange }: { role?: InputRole; values?: Record<string, ImageParameterValue>; parameters: Record<string, InputValue>; onChange: (values: Record<string, ImageParameterValue>) => void }) {
    if (!role?.fields?.length) return null;
    return (
        <div className="space-y-1.5 rounded-md bg-surface-secondary p-2">
            {role.fields.map((field) => (
                <label key={field.name} className="block space-y-1 text-xs text-foreground/70">
                    <span>{field.label}</span>
                    <ModelInputField
                        field={{ ...field, ...(field.upperBoundField ? { max: Number(parameters[field.upperBoundField]) - 0.001 } : {}) }}
                        value={values?.[field.name]}
                        onChange={(value) => onChange({ ...values, [field.name]: value as ImageParameterValue })}
                    />
                </label>
            ))}
        </div>
    );
}

export function ModelInputMediaAssignments({ config, media, value, parameters, onChange }: { config: ModelInputConfig; media: InputMedia[]; value?: string; parameters: Record<string, InputValue>; onChange: (value: string) => void }) {
    const assignments = inputMediaAssignments(value);
    if (!media.length) return null;
    return (
        <div className="space-y-2">
            <div className="text-xs font-medium text-foreground/70">素材用途</div>
            {media
                .filter((item) => item.id && item.kind === "image")
                .map((item, index) => {
                    const assigned = assignments[item.id!];
                    const set = (patch: { role: string; inputFields?: Record<string, ImageParameterValue> }) => {
                        const next = { ...assignments };
                        if (!patch.role) delete next[item.id!];
                        else next[item.id!] = patch;
                        onChange(JSON.stringify(next));
                    };
                    return (
                        <div key={item.id} className="space-y-2 rounded-md border border-border p-2">
                            <div className="text-xs text-foreground/65">{item.label || `参考图 ${index + 1}`}</div>
                            <Select
                                aria-label={`${item.label || `参考图 ${index + 1}`}用途`}
                                className="w-full"
                                value={assigned?.role || ""}
                                options={[{ value: "", label: "自动（沿用当前引用）" }, ...config.mediaRoles.filter((role) => role.kind === item.kind && role.maxCount > 0).map((role) => ({ value: role.role, label: role.label }))]}
                                onChange={(role) => set({ role, inputFields: assigned?.inputFields })}
                            />
                            <ModelMediaFields role={config.mediaRoles.find((role) => role.role === assigned?.role)} values={assigned?.inputFields} parameters={parameters} onChange={(inputFields) => set({ role: assigned!.role, inputFields })} />
                        </div>
                    );
                })}
        </div>
    );
}

export function InputConstraintEditor({ field, onChange, disabled }: { field: InputField; onChange: (field: InputField) => void; disabled?: boolean }) {
    if (field.readOnly) return <div className="model-input-readonly">{typeof field.default === "boolean" ? (field.default ? "开启" : "关闭") : String(field.default ?? "")}</div>;
    if (field.type === "boolean")
        return (
            <label className="model-input-boolean-default">
                默认开启 <Switch aria-label={`${field.label}默认值`} disabled={disabled} checked={field.default === true} onChange={(value) => onChange({ ...field, default: value })} />
            </label>
        );
    if (field.type === "object[]" || field.type === "string[]")
        return (
            <label className="model-input-label model-input-array-limit">
                <span>最多项数（0 为不限）</span>
                <InputNumber aria-label={`${field.label}最多项数`} min={0} precision={0} disabled={disabled} value={field.maxItems} onChange={(value) => onChange({ ...field, maxItems: value || 0 })} />
            </label>
        );
    if (field.type === "string")
        return (
            <div className="model-input-constraint-editor model-input-string-constraints">
                <label className="model-input-label">
                    <span>支持值</span>
                    <Select
                        aria-label={`${field.label}支持值`}
                        mode="tags"
                        className="w-full"
                        disabled={disabled}
                        value={field.values?.map(String) || []}
                        tokenSeparators={[",", "，"]}
                        onChange={(values) => onChange({ ...field, values, default: values.includes(String(field.default)) ? field.default : values[0] })}
                    />
                </label>
                <label className="model-input-label">
                    <span>默认值</span>
                    {field.values?.length ? (
                        <Select
                            aria-label={`${field.label}默认值`}
                            disabled={disabled}
                            value={field.default === undefined ? undefined : String(field.default)}
                            options={field.values.map((value) => ({ value: String(value), label: String(value) }))}
                            onChange={(value) => onChange({ ...field, default: value })}
                        />
                    ) : (
                        <Input aria-label={`${field.label}默认值`} disabled={disabled} value={String(field.default ?? "")} onChange={(event) => onChange({ ...field, default: event.target.value })} />
                    )}
                </label>
            </div>
        );
    return (
        <div className="model-input-constraint-editor model-input-numeric-constraints">
            {(
                [
                    ["min", "最小值"],
                    ["max", "最大值"],
                    ["step", "步长"],
                    ["default", "默认值"],
                ] as const
            ).map(([key, label]) => (
                <label key={key} className="model-input-label">
                    <span>{label}</span>
                    <InputNumber
                        aria-label={`${field.label}${label}`}
                        precision={field.type === "integer" ? 0 : undefined}
                        disabled={disabled}
                        value={field[key] as number | undefined}
                        onChange={(value) => onChange({ ...field, [key]: value ?? undefined })}
                    />
                </label>
            ))}
        </div>
    );
}
