import { App, Button, Input, Skeleton } from "antd";
import { AlertTriangle, Download, ExternalLink, History, RefreshCw, RotateCcw } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "@/services/api/request";
import { checkSystemUpdate, getSystemUpdateStatus, rollbackSystemUpdate, startSystemUpdate, type SystemUpdateStatus, type UpdatePhase } from "@/services/api/system-update";
import { AdminPageFrame } from "../components/admin-shell";
import { AdminStatusBadge, SettingsSectionCard } from "../components/admin-ui";
import { isUpdateActive, needsUpdatedPage, updatePhaseLabels } from "./system-update-state";

export default function SystemUpdatePage() {
    const { message, modal } = App.useApp();
    const [status, setStatus] = useState<SystemUpdateStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [checking, setChecking] = useState(false);
    const [starting, setStarting] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [loadError, setLoadError] = useState("");
    const current = useRef<SystemUpdateStatus | null>(null);
    const mounted = useRef(false);
    const polling = useRef(false);
    const reloadRequested = useRef(false);
    const uncertainSubmission = useRef<{ previousID?: string } | null>(null);

    const acceptStatus = useCallback((next: SystemUpdateStatus) => {
        if (!mounted.current) return;
        current.current = next;
        setStatus(next);
        const submission = uncertainSubmission.current;
        uncertainSubmission.current = null;
        setLoadError(submission && submission.previousID === next.operation.id && !isUpdateActive(next.operation.phase) ? "未能确认操作已启动，请重试；重复提交不会重复安装。" : "");
        setReconnecting(false);
        if (!reloadRequested.current && needsUpdatedPage(next, __APP_VERSION__)) {
            const key = `canvas:program-update:${next.operation.id}`;
            try {
                if (sessionStorage.getItem(key)) return;
                sessionStorage.setItem(key, "reloaded");
            } catch {
                // Without a persistent reload guard, let the user refresh explicitly.
                return;
            }
            reloadRequested.current = true;
            window.location.reload();
        }
    }, []);

    const load = useCallback(async () => {
        if (polling.current) return;
        polling.current = true;
        try {
            acceptStatus(await getSystemUpdateStatus());
        } catch (error) {
            if (!mounted.current) return;
            setLoadError(error instanceof Error ? error.message : "读取更新状态失败");
            if (error instanceof ApiError && (error.status === 401 || error.status === 403)) setReconnecting(false);
            else if (isUpdateActive(current.current?.operation.phase)) setReconnecting(true);
        } finally {
            polling.current = false;
        }
    }, [acceptStatus]);

    useEffect(() => {
        mounted.current = true;
        const controller = new AbortController();
        void (async () => {
            try {
                const initial = await getSystemUpdateStatus(controller.signal);
                if (controller.signal.aborted) return;
                acceptStatus(initial);
                if (!isUpdateActive(initial.operation.phase)) acceptStatus(await checkSystemUpdate(false, controller.signal));
            } catch (error) {
                if (!controller.signal.aborted && mounted.current) {
                    setLoadError(error instanceof Error ? error.message : "检查更新失败");
                    if (!current.current && error instanceof ApiError && error.retryable) setReconnecting(true);
                }
            } finally {
                if (!controller.signal.aborted && mounted.current) setLoading(false);
            }
        })();
        return () => {
            mounted.current = false;
            controller.abort();
        };
    }, [acceptStatus]);

    const active = isUpdateActive(status?.operation.phase);
    useEffect(() => {
        if (!active && !reconnecting) return;
        const timer = window.setInterval(() => void load(), 2500);
        return () => window.clearInterval(timer);
    }, [active, reconnecting, load]);

    const requestCheck = async () => {
        setChecking(true);
        try {
            const next = await checkSystemUpdate(true);
            acceptStatus(next);
            message.success(next.updateAvailable ? `发现新版本 ${next.latestRelease?.version}` : "当前已是最新版本");
        } catch (error) {
            if (mounted.current) setLoadError(error instanceof Error ? error.message : "检查更新失败");
        } finally {
            if (mounted.current) setChecking(false);
        }
    };

    const submit = async (action: () => Promise<SystemUpdateStatus>) => {
        const previousID = current.current?.operation.id;
        setStarting(true);
        try {
            acceptStatus(await action());
        } catch (error) {
            if (error instanceof ApiError && error.retryable) {
                uncertainSubmission.current = { previousID };
                setReconnecting(true);
                await load();
                return;
            }
            message.error(error instanceof Error ? error.message : "无法开始更新");
            throw error;
        } finally {
            if (mounted.current) setStarting(false);
        }
    };

    const requestStart = () => {
        const target = status?.latestRelease?.version;
        if (!target) return;
        modal.confirm({
            title: `更新到 ${target}？`,
            content: "新版程序下载并校验后，会等待现有任务完成，再自动安装和重启。重启期间暂时无法访问，页面会自动重连。",
            okText: "立即更新",
            cancelText: "取消",
            onOk: () => submit(() => startSystemUpdate(target)),
        });
    };

    const requestRollback = () => {
        if (!status?.canRollback) return;
        let reason = "";
        modal.confirm({
            title: `回退到 ${status.rollbackVersion || "上一版本"}？`,
            content: (
                <div className="space-y-3">
                    <p>回退只替换程序，不恢复或删除业务数据。旧程序通过数据库兼容检查后才会安装并重启。</p>
                    <Input.TextArea rows={3} maxLength={300} placeholder="回退原因（必填）" onChange={(event) => (reason = event.target.value)} />
                </div>
            ),
            okText: "回退并重启",
            okButtonProps: { danger: true },
            cancelText: "取消",
            onOk: async () => {
                if (!reason.trim()) {
                    message.warning("请填写回退原因");
                    throw new Error("rollback reason required");
                }
                await submit(() => rollbackSystemUpdate(reason.trim()));
            },
        });
    };

    return (
        <AdminPageFrame
            title="系统更新"
            description="从 GitHub Release 获取新版程序，一键安装并自动重启。"
            scroll
            actions={
                <Button icon={<RefreshCw className="size-4" />} loading={checking} disabled={active || starting || loading} onClick={() => void requestCheck()}>
                    检查更新
                </Button>
            }
        >
            <div className="admin-settings-stack admin-system-update">
                {loading && !status ? <Skeleton active paragraph={{ rows: 8 }} /> : null}
                {loadError && !reconnecting ? <UpdateAlert title="检查或读取更新失败" detail={loadError} /> : null}
                {reconnecting ? <UpdateAlert title="等待服务重新连接" detail="已保留最后进度，连接恢复后会自动读取更新结果。" /> : null}
                {status && !status.supported ? <UpdateAlert title="此部署只能检查版本" detail={status.unsupportedReason || "请使用官方发布程序和持久化程序目录进行网页更新。"} /> : null}
                {status ? (
                    <>
                        <SettingsSectionCard
                            layout="stacked"
                            icon={<Download className="size-4" />}
                            title="版本更新"
                            status={<PhaseBadge phase={status.operation.phase} />}
                            footer={
                                <>
                                    <span className="text-sm text-foreground/65">{status.checkedAt ? `上次检查：${formatDate(status.checkedAt)}` : "尚未检查版本"}</span>
                                    <Button type="primary" loading={starting} disabled={!status.supported || !status.updateAvailable || active || checking || reconnecting || loading} onClick={requestStart}>
                                        立即更新
                                    </Button>
                                </>
                            }
                        >
                            <div className="admin-system-update-release-body">
                                <div className="admin-system-update-versions">
                                    <div>
                                        <span>当前版本</span>
                                        <strong>{status.currentVersion}</strong>
                                    </div>
                                    <div>
                                        <span>最新稳定版</span>
                                        <strong>{status.latestRelease?.version || "尚未获取"}</strong>
                                        {status.updateAvailable ? <AdminStatusBadge label="可更新" tone="success" /> : null}
                                    </div>
                                </div>
                                {status.latestRelease ? (
                                    <div className="admin-system-update-notes">
                                        <div className="flex items-center justify-between gap-3">
                                            <h3>更新内容</h3>
                                            <a href={status.latestRelease.url} target="_blank" rel="noreferrer">
                                                查看 Release <ExternalLink className="size-3" />
                                            </a>
                                        </div>
                                        <p className="text-sm text-foreground/65">{formatDate(status.latestRelease.publishedAt)}</p>
                                        <pre>{status.latestRelease.body || "此版本未提供更新说明。"}</pre>
                                    </div>
                                ) : null}
                                {needsUpdatedPage(status, __APP_VERSION__) ? (
                                    <Button icon={<RefreshCw className="size-4" />} onClick={() => window.location.reload()}>
                                        刷新页面以加载当前版本
                                    </Button>
                                ) : null}
                            </div>
                        </SettingsSectionCard>
                        {status.operation.id ? (
                            <SettingsSectionCard layout="stacked" icon={<History className="size-4" />} title={status.operation.kind === "rollback" ? "回退进度" : "更新进度"} status={<PhaseBadge phase={status.operation.phase} />}>
                                <div className="admin-system-update-progress" aria-live="polite">
                                    {status.operation.error ? <UpdateAlert title={status.operation.phase === "manual_intervention" ? "需要恢复程序" : "操作未完成"} detail={status.operation.error} /> : null}
                                    <ol>
                                        {status.operation.logs.map((entry, index) => (
                                            <li key={`${entry.at}-${index}`}>
                                                <span className="admin-system-update-dot" data-active={active && index === status.operation.logs.length - 1} />
                                                <div>
                                                    <strong>{updatePhaseLabels[entry.phase]}</strong>
                                                    <p>{entry.message}</p>
                                                </div>
                                                <time>{formatDate(entry.at)}</time>
                                            </li>
                                        ))}
                                    </ol>
                                </div>
                            </SettingsSectionCard>
                        ) : null}
                        {status.canRollback ? (
                            <SettingsSectionCard
                                layout="stacked"
                                icon={<RotateCcw className="size-4" />}
                                title="上一版程序"
                                description={`已保留 ${status.rollbackVersion || "上一版本"}。回退前会检查数据库兼容性。`}
                                footer={
                                    <Button danger icon={<RotateCcw className="size-4" />} disabled={active || starting || reconnecting || !status.supported} onClick={requestRollback}>
                                        回退到上一版本
                                    </Button>
                                }
                            >
                                <p className="text-sm text-foreground/65">只恢复程序，不覆盖当前数据库或创作文件。</p>
                            </SettingsSectionCard>
                        ) : null}
                    </>
                ) : null}
            </div>
        </AdminPageFrame>
    );
}

function PhaseBadge({ phase }: { phase: UpdatePhase }) {
    const tone = phase === "succeeded" ? "success" : phase === "failed" || phase === "manual_intervention" ? "error" : phase === "rolled_back" ? "warning" : isUpdateActive(phase) ? "info" : "neutral";
    return <AdminStatusBadge label={updatePhaseLabels[phase]} tone={tone} />;
}

function UpdateAlert({ title, detail }: { title: string; detail: string }) {
    return (
        <div className="admin-system-update-alert" role="status">
            <AlertTriangle className="size-4" />
            <div>
                <strong>{title}</strong>
                <p>{detail}</p>
            </div>
        </div>
    );
}

function formatDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}
