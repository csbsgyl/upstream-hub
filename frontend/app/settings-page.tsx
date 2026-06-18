"use client"

import { useMemo, useState } from "react"
import { toast } from "sonner"
import {
  Activity,
  AlertTriangle,
  BellRing,
  CheckCircle2,
  ClipboardCopy,
  ArrowUpRight,
  DatabaseBackup,
  Download,
  FileJson,
  Gauge,
  HardDrive,
  Loader2,
  Play,
  RotateCcw,
  Sparkles,
  Trash2,
  RefreshCw,
} from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useConfirm } from "@/components/ui/confirm-dialog"
import { apiFetch, getToken } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import { useAuditLogs, useFailedNotificationLogs, useOpsStatus, useVersionCheck } from "@/lib/queries"
import { useTriggerRefresh } from "@/lib/refresh-context"
import { relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { ComponentType, ReactNode } from "react"
import type { BackupState, OpsBackupResponse, OpsRetentionResult, OpsScanResult, OpsUpdateResult } from "@/lib/api-types"

type BusyAction =
  | "backup"
  | "diagnostics"
  | "copy"
  | "retention"
  | "scan-sync"
  | "scan-balances"
  | "scan-rates"
  | "copy-update"
  | "update"
  | `download-${string}`

function asText(v: unknown, fallback = "-") {
  if (v == null || v === "") return fallback
  if (Array.isArray(v)) return v.length === 0 ? "-" : v.join(", ")
  if (typeof v === "boolean") return v ? "开启" : "关闭"
  return String(v)
}

function asNumber(v: unknown, fallback = 0) {
  if (typeof v === "number" && Number.isFinite(v)) return v
  if (typeof v === "string") {
    const n = Number(v)
    if (Number.isFinite(n)) return n
  }
  return fallback
}

function retentionValue(retention: unknown, camelKey: string, pascalKey: string, fallback: string) {
  if (!retention || typeof retention !== "object") return fallback
  const record = retention as Record<string, unknown>
  return asText(record[camelKey] ?? record[pascalKey], fallback)
}

function boolTone(ok: boolean) {
  return ok ? "bg-success/10 text-success ring-success/20" : "bg-danger/10 text-danger ring-danger/20"
}

function formatSize(bytes?: number) {
  if (!bytes || bytes <= 0) return "-"
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-medium text-foreground">{value}</span>
    </div>
  )
}

type Tone = "brand" | "success" | "warning" | "danger" | "muted"

function toneClass(tone: Tone) {
  switch (tone) {
    case "brand":
      return {
        icon: "bg-brand/10 text-brand ring-brand/15",
        value: "text-brand",
      }
    case "success":
      return {
        icon: "bg-success/10 text-success ring-success/20",
        value: "text-success",
      }
    case "warning":
      return {
        icon: "bg-warning/10 text-warning ring-warning/20",
        value: "text-warning",
      }
    case "danger":
      return {
        icon: "bg-danger/10 text-danger ring-danger/20",
        value: "text-danger",
      }
    default:
      return {
        icon: "bg-muted text-muted-foreground ring-border",
        value: "text-foreground",
      }
  }
}

function StatTile({
  icon: Icon,
  label,
  value,
  detail,
  tone = "muted",
}: {
  icon: ComponentType<{ className?: string }>
  label: string
  value: ReactNode
  detail?: ReactNode
  tone?: Tone
}) {
  const cls = toneClass(tone)
  return (
    <div className="min-w-0 rounded-md border border-border bg-background px-3 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className={cn("mt-1 truncate text-xl font-semibold tracking-normal", cls.value)}>{value}</p>
        </div>
        <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-md ring-1", cls.icon)}>
          <Icon className="size-4" />
        </span>
      </div>
      {detail ? <p className="mt-2 truncate text-xs text-muted-foreground">{detail}</p> : null}
    </div>
  )
}

function ActionButton({
  busy,
  busyKey,
  children,
  className,
  disabled,
  onClick,
  variant = "default",
}: {
  busy: BusyAction | null
  busyKey: BusyAction
  children: ReactNode
  className?: string
  disabled?: boolean
  onClick: () => void
  variant?: React.ComponentProps<typeof Button>["variant"]
}) {
  const active = busy === busyKey
  return (
    <Button
      type="button"
      variant={variant}
      size="sm"
      className={cn("gap-1.5", className)}
      disabled={disabled || active}
      onClick={onClick}
    >
      {active ? <Loader2 className="size-3.5 animate-spin" /> : null}
      {children}
    </Button>
  )
}

export default function SettingsPage() {
  const { username } = useAuth()
  const status = useOpsStatus()
  const version = useVersionCheck()
  const audits = useAuditLogs(80)
  const failedNotifications = useFailedNotificationLogs(50)
  const refresh = useTriggerRefresh()
  const [busyRetry, setBusyRetry] = useState<number | null>(null)
  const [busy, setBusy] = useState<BusyAction | null>(null)
  const [lastRetention, setLastRetention] = useState<OpsRetentionResult | null>(null)
  const [lastUpdate, setLastUpdate] = useState<OpsUpdateResult | null>(null)
  const { confirm, dialog } = useConfirm()

  const s = status.data
  const latestBackup = s?.backups?.[0] ?? null
  const failed = failedNotifications.data ?? []
  const monitorEnabled = asNumber(s?.channels?.monitor_enabled)
  const totalChannels = asNumber(s?.channels?.total)
  const failedMonitorLogs = asNumber(s?.channels?.failed)
  const rateSnapshots = asNumber(s?.channels?.rate_snapshots)
  const rateChanges = asNumber(s?.channels?.rate_changes)
  const notifyEnabled = asNumber(s?.notifications?.enabled)
  const notifyTotal = asNumber(s?.notifications?.total)
  const failedNotifyCount = asNumber(s?.notifications?.failed_notification_logs)
  const backupCount = s?.backups?.length ?? 0
  const systemLoading = status.loading && !s
  const systemReady = Boolean(s && s.database === "ok" && s.app_secret_ready)
  const versionInfo = version.data
  const versionError = version.error || versionInfo?.check_error || ""
  const versionTitle = version.loading
    ? "正在检查版本"
    : versionError
      ? "版本检查失败"
      : versionInfo?.has_update
        ? "检测到新版本"
        : "当前已是最新"
  const versionBadge = version.loading ? "检查中" : versionError ? "检查失败" : versionInfo?.has_update ? "可更新" : "无需更新"
  const autoUpdate = versionInfo?.auto_update
  const autoUpdateReady = Boolean(versionInfo?.has_update && autoUpdate?.available)

  const diagnosticsSummary = useMemo(() => {
    if (!s) return ""
    return [
      `生成时间: ${new Date(s.generated_at).toLocaleString("zh-CN")}`,
      `数据库: ${s.database}`,
      `鉴权: ${s.auth_enabled ? "开启" : "关闭"}`,
      `APP_SECRET: ${s.app_secret_ready ? "已配置" : "缺失"}`,
      `启用监控渠道: ${asNumber(s.channels?.monitor_enabled)} / ${asNumber(s.channels?.total)}`,
      `监控失败记录: ${asNumber(s.channels?.failed)}`,
      `倍率快照: ${asNumber(s.channels?.rate_snapshots)}`,
      `倍率变动: ${asNumber(s.channels?.rate_changes)}`,
      `启用通知渠道: ${asNumber(s.notifications?.enabled)} / ${asNumber(s.notifications?.total)}`,
      `失败通知: ${asNumber(s.notifications?.failed_notification_logs)}`,
      `最近备份: ${latestBackup ? `${latestBackup.name} (${relativeTime(latestBackup.updated_at)})` : "未发现"}`,
    ].join("\n")
  }, [latestBackup, s])

  function reloadOps() {
    status.refetch()
    version.refetch()
    audits.refetch()
    failedNotifications.refetch()
    refresh()
  }

  async function runAction<T>(key: BusyAction, fn: () => Promise<T>) {
    setBusy(key)
    try {
      return await fn()
    } finally {
      setBusy(null)
    }
  }

  async function retryLog(id: number) {
    setBusyRetry(id)
    try {
      const res = await apiFetch<{ ok: boolean; error?: string }>(`/notifications/logs/${id}/retry`, {
        method: "POST",
      })
      if (res.ok) {
        toast.success("通知已重新发送")
      } else {
        toast.error(res.error ?? "重发失败")
      }
      reloadOps()
    } catch (e) {
      const err = e as Error
      toast.error(err.message || "重发失败")
    } finally {
      setBusyRetry(null)
    }
  }

  async function createBackup() {
    await runAction("backup", async () => {
      const res = await apiFetch<OpsBackupResponse>("/ops/backups", { method: "POST" })
      toast.success(`备份已生成：${res.backup.name}`)
      reloadOps()
    }).catch((e: Error) => toast.error(e.message || "备份失败"))
  }

  async function downloadBackup(file: BackupState) {
    await runAction(`download-${file.name}`, async () => {
      const token = getToken()
      const headers = new Headers()
      if (token) headers.set("Authorization", `Bearer ${token}`)
      const res = await fetch(`/api/ops/backups/${encodeURIComponent(file.name)}/download`, { headers })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `下载失败：HTTP ${res.status}`)
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = file.name
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      toast.success("备份已开始下载")
    }).catch((e: Error) => toast.error(e.message || "下载失败"))
  }

  async function downloadDiagnostics() {
    await runAction("diagnostics", async () => {
      const data = await apiFetch<Record<string, unknown>>("/ops/diagnostics")
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `upstream-hub-diagnostics-${new Date().toISOString().replace(/[:.]/g, "-")}.json`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      toast.success("诊断包已生成")
    }).catch((e: Error) => toast.error(e.message || "诊断包下载失败"))
  }

  async function copyDiagnostics() {
    await runAction("copy", async () => {
      if (!diagnosticsSummary) throw new Error("状态还没有加载完成")
      await navigator.clipboard.writeText(diagnosticsSummary)
      toast.success("诊断摘要已复制")
    }).catch((e: Error) => toast.error(e.message || "复制失败"))
  }

  async function copyUpdateCommand() {
    await runAction("copy-update", async () => {
      const command = versionInfo?.update_command ?? "git pull && ./scripts/deploy.sh"
      await navigator.clipboard.writeText(command)
      toast.success("更新命令已复制")
    }).catch((e: Error) => toast.error(e.message || "复制失败"))
  }

  async function runUpdate() {
    if (!autoUpdate?.available) {
      toast.error(autoUpdate?.reason || "当前部署环境不支持网页一键更新")
      return
    }
    const ok = await confirm({
      title: "立即执行服务器更新？",
      description:
        "系统会在后台拉取最新代码、备份数据库并重建 Docker 服务。更新期间页面可能短暂断开，完成后刷新页面即可。",
      confirmLabel: "开始更新",
      cancelLabel: "取消",
    })
    if (!ok) return
    await runAction("update", async () => {
      const res = await apiFetch<OpsUpdateResult>("/ops/update", { method: "POST" })
      setLastUpdate(res)
      toast.success(res.message || "更新任务已启动", {
        description: res.log_file ? `日志：${res.log_file}` : undefined,
        duration: 9000,
      })
      reloadOps()
    }).catch((e: Error) => toast.error(e.message || "更新启动失败"))
  }

  async function scan(job: "sync" | "balances" | "rates") {
    const key = job === "sync" ? "scan-sync" : job === "balances" ? "scan-balances" : "scan-rates"
    await runAction(key, async () => {
      const res = await apiFetch<OpsScanResult>(`/ops/scan/${job}`, { method: "POST" })
      if (!res.started) {
        toast.warning(res.message || "任务没有启动")
        return
      }
      toast.success(res.message || "后台任务已启动")
      reloadOps()
    }).catch((e: Error) => toast.error(e.message || "扫描启动失败"))
  }

  async function runRetention() {
    await runAction("retention", async () => {
      const res = await apiFetch<OpsRetentionResult>("/ops/retention/run", { method: "POST" })
      setLastRetention(res)
      toast.success(
        `已清理 ${res.monitor_logs_deleted + res.balance_snapshots_deleted + res.notification_logs_deleted} 条历史记录`,
      )
      reloadOps()
    }).catch((e: Error) => toast.error(e.message || "日志清理失败"))
  }

  return (
    <section className="space-y-3">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold text-foreground">运维中心</h1>
          <p className="text-xs text-muted-foreground">
            备份、手动同步、失败补发、清理和诊断集中在这里处理。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <ActionButton busy={busy} busyKey="backup" onClick={createBackup}>
            <DatabaseBackup className="size-3.5" />
            立即备份
          </ActionButton>
          <ActionButton busy={busy} busyKey="diagnostics" variant="outline" onClick={downloadDiagnostics}>
            <Download className="size-3.5" />
            下载诊断包
          </ActionButton>
          <ActionButton busy={busy} busyKey="copy" variant="outline" onClick={copyDiagnostics}>
            <ClipboardCopy className="size-3.5" />
            复制摘要
          </ActionButton>
        </div>
      </header>

      <Card className="border border-border shadow-none">
        <CardContent className="p-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">系统更新</p>
              <h2 className="mt-1 text-base font-semibold text-foreground">
                {versionTitle}
              </h2>
              <p className="mt-1 text-xs text-muted-foreground">
                {versionInfo
                  ? `当前 ${versionInfo.current.short_commit} · 最新 ${versionInfo.latest_short ?? versionInfo.current.short_commit}`
                  : "正在从 GitHub 检查当前仓库的最新提交"}
              </p>
            </div>
            <Badge
              className={cn(
                "w-fit ring-1",
                versionError
                  ? "bg-danger/10 text-danger ring-danger/20"
                  : versionInfo?.has_update
                  ? "bg-warning/10 text-warning ring-warning/20"
                  : "bg-success/10 text-success ring-success/20",
              )}
            >
              {versionBadge}
            </Badge>
          </div>

          <div className="mt-3 grid gap-2 md:grid-cols-4">
            <StatTile
              icon={RefreshCw}
              label="当前版本"
              value={versionInfo?.current.short_commit ?? "-"}
              detail={versionInfo?.current.branch ?? "-"}
              tone="brand"
            />
            <StatTile
              icon={Gauge}
              label="最新版本"
              value={versionInfo?.latest_short ?? "-"}
              detail={versionInfo?.checked_at ? relativeTime(versionInfo.checked_at) : "未检查"}
              tone={versionInfo?.has_update ? "warning" : "success"}
            />
            <StatTile
              icon={ArrowUpRight}
              label="更新状态"
              value={
                versionError
                  ? "未知"
                  : versionInfo?.has_update
                    ? autoUpdate?.available
                      ? "可一键更新"
                      : "需先升级环境"
                    : "最新"
              }
              detail={
                versionError ||
                (versionInfo?.has_update
                  ? autoUpdate?.available
                    ? "已具备网页更新能力"
                    : autoUpdate?.reason || "当前部署环境未启用一键更新"
                  : "无需操作")
              }
              tone={versionError || (versionInfo?.has_update && !autoUpdate?.available) ? "danger" : versionInfo?.has_update ? "warning" : "success"}
            />
            <StatTile
              icon={FileJson}
              label="仓库"
              value={versionInfo?.current.repository ?? "csbsgyl/upstream-hub"}
              detail={versionInfo?.update_url ?? "https://github.com/csbsgyl/upstream-hub"}
              tone="muted"
            />
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <ActionButton
              busy={busy}
              busyKey="update"
              className="gap-1.5"
              disabled={!autoUpdateReady}
              onClick={runUpdate}
            >
              <RefreshCw className="size-3.5" />
              立即更新
            </ActionButton>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              disabled={version.loading}
              onClick={() => version.refetch()}
            >
              <RefreshCw className={cn("size-3.5", version.loading && "animate-spin")} />
              重新检查
            </Button>
            <ActionButton busy={busy} busyKey="copy-update" variant="outline" onClick={copyUpdateCommand}>
              <ClipboardCopy className="size-3.5" />
              复制备用命令
            </ActionButton>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              asChild
            >
              <a href={versionInfo?.update_url ?? "https://github.com/csbsgyl/upstream-hub"} target="_blank" rel="noreferrer">
                <ArrowUpRight className="size-3.5" />
                打开仓库
              </a>
            </Button>
            <span className="text-xs text-muted-foreground">
              {versionInfo?.compare_url ? (
                <a className="underline underline-offset-2 hover:text-foreground" href={versionInfo.compare_url} target="_blank" rel="noreferrer">
                  查看差异
                </a>
              ) : (
                "没有可比较的提交信息"
              )}
            </span>
          </div>
          {versionInfo?.has_update && !autoUpdate?.available ? (
            <p className="mt-2 rounded-md border border-danger/20 bg-danger/5 px-3 py-2 text-xs leading-relaxed text-danger">
              一键更新暂不可用：{autoUpdate?.reason || "当前部署环境缺少自动更新能力"}。先在服务器执行一次备用更新命令，升级后以后就可以直接点“立即更新”。
            </p>
          ) : lastUpdate ? (
            <p className="mt-2 rounded-md border border-success/20 bg-success/5 px-3 py-2 text-xs leading-relaxed text-success">
              更新任务已提交：{lastUpdate.container_name}，日志文件 {lastUpdate.log_file}。服务重建时页面可能短暂断开。
            </p>
          ) : null}
        </CardContent>
      </Card>

      <Card className="border border-border shadow-none">
        <CardContent className="p-4">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-center">
            <div className="flex min-w-0 flex-1 items-start gap-3">
              <span
                className={cn(
                  "flex size-11 shrink-0 items-center justify-center rounded-md ring-1",
                  systemLoading
                    ? "bg-muted text-muted-foreground ring-border"
                    : systemReady
                      ? "bg-success/10 text-success ring-success/20"
                      : "bg-warning/10 text-warning ring-warning/20",
                )}
              >
                {systemLoading ? (
                  <Loader2 className="size-5 animate-spin" />
                ) : systemReady ? (
                  <CheckCircle2 className="size-5" />
                ) : (
                  <AlertTriangle className="size-5" />
                )}
              </span>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-base font-semibold text-foreground">
                    {systemLoading ? "正在读取系统状态" : systemReady ? "系统运行正常" : "系统需要关注"}
                  </h2>
                  {systemLoading ? (
                    <Badge variant="outline">加载中</Badge>
                  ) : (
                    <>
                      <Badge className={cn("ring-1", boolTone(s?.database === "ok"))}>
                        数据库 {s?.database ?? "检查中"}
                      </Badge>
                      <Badge className={cn("ring-1", boolTone(Boolean(s?.app_secret_ready)))}>
                        APP_SECRET {s?.app_secret_ready ? "已配置" : "缺失"}
                      </Badge>
                    </>
                  )}
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                  {systemLoading
                    ? "正在连接后端并拉取最新运维摘要"
                    : `当前账号 ${username ?? "-"}，状态生成于 ${relativeTime(s?.generated_at)}`}
                </p>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2 md:grid-cols-4 xl:w-[720px]">
              <StatTile
                icon={Gauge}
                label="监控渠道"
                value={`${monitorEnabled}/${totalChannels}`}
                detail={failedMonitorLogs > 0 ? `${failedMonitorLogs} 条失败记录` : "采集正常"}
                tone={failedMonitorLogs > 0 ? "danger" : "success"}
              />
              <StatTile
                icon={Activity}
                label="同步频率"
                value={asText(s?.scheduler?.sync_cron)}
                detail={`并发 ${asText(s?.scheduler?.concurrency, "1")}`}
                tone="brand"
              />
              <StatTile
                icon={BellRing}
                label="通知渠道"
                value={`${notifyEnabled}/${notifyTotal}`}
                detail={failedNotifyCount > 0 ? `${failedNotifyCount} 条发送失败` : "无失败通知"}
                tone={failedNotifyCount > 0 ? "danger" : "success"}
              />
              <StatTile
                icon={HardDrive}
                label="倍率追踪"
                value={rateChanges}
                detail={`${rateSnapshots} 个快照 · ${backupCount} 个备份`}
                tone={rateChanges > 0 ? "warning" : "muted"}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="space-y-3">
          <Card className="border border-border shadow-none">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <DatabaseBackup className="size-4 text-warning" />
                数据库备份
              </CardTitle>
              <Badge variant="outline">{backupCount}</Badge>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-2 rounded-md border border-border bg-muted/25 p-3 md:grid-cols-3">
                <div className="min-w-0">
                  <p className="text-xs text-muted-foreground">最新备份</p>
                  <p className="mt-1 truncate text-sm font-medium text-foreground">
                    {latestBackup ? latestBackup.name : "未发现"}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">备份时间</p>
                  <p className="mt-1 text-sm font-medium text-foreground">
                    {latestBackup ? relativeTime(latestBackup.updated_at) : "-"}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">文件大小</p>
                  <p className="mt-1 text-sm font-medium text-foreground">
                    {latestBackup ? formatSize(latestBackup.size) : "-"}
                  </p>
                </div>
              </div>
              {!s?.backups?.length ? (
                <p className="text-xs text-muted-foreground">
                  还没有可下载的数据库备份。点击“立即备份”会生成新的 .sql.gz 文件。
                </p>
              ) : (
                <ScrollArea className="h-44">
                  <ul className="divide-y divide-border">
                    {s.backups.map((file) => (
                      <li key={file.name} className="flex items-center justify-between gap-3 py-2">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium text-foreground">{file.name}</p>
                          <p className="mt-0.5 text-[11px] text-muted-foreground">
                            {formatSize(file.size)}
                            {" · "}
                            {relativeTime(file.updated_at)}
                          </p>
                        </div>
                        <ActionButton
                          busy={busy}
                          busyKey={`download-${file.name}`}
                          variant="outline"
                          className="h-7 shrink-0 text-xs"
                          onClick={() => downloadBackup(file)}
                        >
                          <Download className="size-3" />
                          下载
                        </ActionButton>
                      </li>
                    ))}
                  </ul>
                </ScrollArea>
              )}
            </CardContent>
          </Card>

          <Card className="border border-border shadow-none">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <FileJson className="size-4 text-muted-foreground" />
                操作审计
              </CardTitle>
              <Badge variant="outline">{audits.data?.length ?? 0}</Badge>
            </CardHeader>
            <CardContent className="px-0">
              {audits.loading ? (
                <p className="px-6 py-4 text-xs text-muted-foreground">加载中...</p>
              ) : !audits.data || audits.data.length === 0 ? (
                <p className="px-6 py-4 text-xs text-muted-foreground">暂无审计记录。</p>
              ) : (
                <ScrollArea className="h-72">
                  <ul className="divide-y divide-border">
                    {audits.data.map((log) => (
                      <li key={log.id} className="px-6 py-3">
                        <div className="flex items-center justify-between gap-3">
                          <p className="truncate text-sm font-medium text-foreground">{log.summary}</p>
                          <span className="shrink-0 text-[11px] text-muted-foreground">{relativeTime(log.created_at)}</span>
                        </div>
                        <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                          {log.actor}
                          {" · "}
                          {log.action}
                          {" · "}
                          {log.resource_type}
                          {log.resource_id ? ` #${log.resource_id}` : ""}
                        </p>
                      </li>
                    ))}
                  </ul>
                </ScrollArea>
              )}
            </CardContent>
          </Card>
        </div>

        <aside className="space-y-3 xl:sticky xl:top-20 xl:self-start">
          <Card className="border border-border shadow-none">
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <Sparkles className="size-4 text-brand" />
                快速操作
              </CardTitle>
            </CardHeader>
            <CardContent className="grid gap-2">
              <ActionButton
                busy={busy}
                busyKey="scan-sync"
                className="h-9"
                disabled={monitorEnabled === 0}
                onClick={() => scan("sync")}
              >
                <Play className="size-3.5" />
                立即同步余额和倍率
              </ActionButton>
              <div className="grid grid-cols-2 gap-2">
                <ActionButton busy={busy} busyKey="scan-balances" variant="outline" disabled={monitorEnabled === 0} onClick={() => scan("balances")}>
                  <Play className="size-3.5" />
                  只扫余额
                </ActionButton>
                <ActionButton busy={busy} busyKey="scan-rates" variant="outline" disabled={monitorEnabled === 0} onClick={() => scan("rates")}>
                  <Play className="size-3.5" />
                  只扫倍率
                </ActionButton>
              </div>
              <ActionButton busy={busy} busyKey="retention" variant="outline" onClick={runRetention}>
                <Trash2 className="size-3.5" />
                执行日志清理
              </ActionButton>
            </CardContent>
          </Card>

          <Card className="border border-border shadow-none">
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <Activity className="size-4 text-brand" />
                调度与保留
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-1">
              <Row label="同步定时" value={asText(s?.scheduler?.sync_cron)} />
              <Row label="余额定时" value={asText(s?.scheduler?.balance_cron)} />
              <Row label="倍率定时" value={asText(s?.scheduler?.rate_cron)} />
              <Row label="倍率合并" value={asText(s?.notifications?.batch_rate_changes)} />
              <Row label="最小变动" value={`${asText(s?.notifications?.min_change_pct, "0")}%`} />
              <Row label="监控日志" value={`${retentionValue(s?.scheduler?.retention, "monitorLogsDays", "MonitorLogsDays", "30")} 天`} />
              <Row label="余额采样" value={`${retentionValue(s?.scheduler?.retention, "balanceSnapshotsDays", "BalanceSnapshotsDays", "90")} 天`} />
              <Row label="通知日志" value={`${retentionValue(s?.scheduler?.retention, "notificationLogsDays", "NotificationLogsDays", "90")} 天`} />
              <Row label="上次清理" value={lastRetention ? relativeTime(lastRetention.ran_at) : "本页未执行"} />
            </CardContent>
          </Card>

          <Card className="border border-border shadow-none">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="flex items-center gap-2 text-base font-semibold">
                <BellRing className="size-4 text-danger" />
                失败通知补发
              </CardTitle>
              <div className="flex items-center gap-2">
                <Badge variant="outline">{failed.length}</Badge>
                <Button size="sm" variant="outline" className="h-7 gap-1 text-xs" onClick={() => failedNotifications.refetch()}>
                  <RotateCcw className="size-3" />
                  刷新
                </Button>
              </div>
            </CardHeader>
            <CardContent className="px-0">
              {failedNotifications.loading ? (
                <p className="px-6 py-4 text-xs text-muted-foreground">加载中...</p>
              ) : failed.length === 0 ? (
                <p className="px-6 py-4 text-xs text-muted-foreground">暂无失败通知。</p>
              ) : (
                <ScrollArea className="h-64">
                  <ul className="divide-y divide-border">
                    {failed.map((log) => (
                      <li key={log.id} className="px-6 py-3">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="truncate text-sm font-medium text-foreground">{log.subject}</p>
                            <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                              {log.event}
                              {" · "}
                              {relativeTime(log.sent_at)}
                            </p>
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 shrink-0 gap-1 text-xs"
                            disabled={busyRetry === log.id}
                            onClick={() => retryLog(log.id)}
                          >
                            <RotateCcw className={cn("size-3", busyRetry === log.id && "animate-spin")} />
                            重发
                          </Button>
                        </div>
                        {log.error_message ? (
                          <p className="mt-2 line-clamp-2 text-[11px] leading-relaxed text-danger">
                            {log.error_message}
                          </p>
                        ) : null}
                      </li>
                    ))}
                  </ul>
                </ScrollArea>
              )}
            </CardContent>
          </Card>
        </aside>
      </div>
      {dialog}
    </section>
  )
}
