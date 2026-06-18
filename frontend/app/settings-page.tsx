"use client"

import { useMemo, useState } from "react"
import { toast } from "sonner"
import {
  Activity,
  BellRing,
  ClipboardCopy,
  DatabaseBackup,
  Download,
  FileJson,
  Gauge,
  Loader2,
  Play,
  RotateCcw,
  ShieldCheck,
  Sparkles,
  Trash2,
} from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { apiFetch, getToken } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import { useAuditLogs, useFailedNotificationLogs, useOpsStatus } from "@/lib/queries"
import { useTriggerRefresh } from "@/lib/refresh-context"
import { relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { BackupState, OpsBackupResponse, OpsRetentionResult, OpsScanResult } from "@/lib/api-types"

type BusyAction =
  | "backup"
  | "diagnostics"
  | "copy"
  | "retention"
  | "scan-sync"
  | "scan-balances"
  | "scan-rates"
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

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-medium text-foreground">{value}</span>
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
  children: React.ReactNode
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
  const audits = useAuditLogs(80)
  const failedNotifications = useFailedNotificationLogs(50)
  const refresh = useTriggerRefresh()
  const [busyRetry, setBusyRetry] = useState<number | null>(null)
  const [busy, setBusy] = useState<BusyAction | null>(null)
  const [lastRetention, setLastRetention] = useState<OpsRetentionResult | null>(null)

  const s = status.data
  const latestBackup = s?.backups?.[0] ?? null
  const failed = failedNotifications.data ?? []
  const monitorEnabled = asNumber(s?.channels?.monitor_enabled)
  const failedMonitorLogs = asNumber(s?.channels?.failed)

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
            备份、手动扫描、通知补发、日志清理和诊断导出集中在这里执行。
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

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-4">
        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <ShieldCheck className="size-4 text-success" />
              系统自检
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="数据库" value={<Badge className={cn("ring-1", boolTone(s?.database === "ok"))}>{s?.database ?? "检查中"}</Badge>} />
            <Row label="鉴权" value={<Badge variant="outline">{s ? (s.auth_enabled ? "已开启" : "未开启") : "-"}</Badge>} />
            <Row label="APP_SECRET" value={<Badge className={cn("ring-1", boolTone(Boolean(s?.app_secret_ready)))}>{s?.app_secret_ready ? "已配置" : "缺失"}</Badge>} />
            <Row label="当前账号" value={username ?? "-"} />
            <Row label="状态生成" value={relativeTime(s?.generated_at)} />
          </CardContent>
        </Card>

        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <Activity className="size-4 text-brand" />
              调度与通知
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="同步定时" value={asText(s?.scheduler?.sync_cron)} />
            <Row label="余额定时" value={asText(s?.scheduler?.balance_cron)} />
            <Row label="倍率定时" value={asText(s?.scheduler?.rate_cron)} />
            <Row label="倍率合并" value={asText(s?.notifications?.batch_rate_changes)} />
            <Row label="最小变动" value={`${asText(s?.notifications?.min_change_pct, "0")}%`} />
            <Row label="失败通知" value={asText(s?.notifications?.failed_notification_logs, "0")} />
          </CardContent>
        </Card>

        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <Gauge className="size-4 text-warning" />
              采集状态
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="启用渠道" value={monitorEnabled} />
            <Row label="失败记录" value={<span className={failedMonitorLogs > 0 ? "text-danger" : ""}>{failedMonitorLogs}</span>} />
            <Row label="倍率快照" value={asText(s?.channels?.rate_snapshots, "0")} />
            <Row label="倍率变动" value={asText(s?.channels?.rate_changes, "0")} />
            <Row label="通知渠道" value={`${asNumber(s?.notifications?.enabled)} / ${asNumber(s?.notifications?.total)}`} />
          </CardContent>
        </Card>

        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <Sparkles className="size-4 text-brand" />
              快速操作
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2">
            <ActionButton busy={busy} busyKey="scan-sync" disabled={monitorEnabled === 0} onClick={() => scan("sync")}>
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
      </div>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="border border-border shadow-none">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <DatabaseBackup className="size-4 text-warning" />
              数据库备份
            </CardTitle>
            <Badge variant="outline">{s?.backups?.length ?? 0}</Badge>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
              <Row label="最新备份" value={latestBackup ? latestBackup.name : "未发现"} />
              <Row label="备份时间" value={latestBackup ? relativeTime(latestBackup.updated_at) : "-"} />
              <Row label="文件大小" value={latestBackup ? formatSize(latestBackup.size) : "-"} />
            </div>
            {!s?.backups?.length ? (
              <p className="text-xs text-muted-foreground">还没有可下载的数据库备份。点击“立即备份”会生成新的 .sql.gz 文件。</p>
            ) : (
              <ScrollArea className="h-56">
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
              <ScrollArea className="h-80">
                <ul className="divide-y divide-border">
                  {failed.map((log) => (
                    <li key={log.id} className="flex items-center justify-between gap-3 px-6 py-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium text-foreground">{log.subject}</p>
                        <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                          {log.event}
                          {" · "}
                          {relativeTime(log.sent_at)}
                          {log.error_message ? ` · ${log.error_message}` : ""}
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
                    </li>
                  ))}
                </ul>
              </ScrollArea>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <Trash2 className="size-4 text-muted-foreground" />
              日志清理结果
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="监控日志保留" value={`${retentionValue(s?.scheduler?.retention, "monitorLogsDays", "MonitorLogsDays", "30")} 天`} />
            <Row label="余额采样保留" value={`${retentionValue(s?.scheduler?.retention, "balanceSnapshotsDays", "BalanceSnapshotsDays", "90")} 天`} />
            <Row label="通知日志保留" value={`${retentionValue(s?.scheduler?.retention, "notificationLogsDays", "NotificationLogsDays", "90")} 天`} />
            <Row label="上次手动清理" value={lastRetention ? relativeTime(lastRetention.ran_at) : "本页未执行"} />
            <Row label="已删监控日志" value={lastRetention?.monitor_logs_deleted ?? "-"} />
            <Row label="已删余额采样" value={lastRetention?.balance_snapshots_deleted ?? "-"} />
            <Row label="已删通知日志" value={lastRetention?.notification_logs_deleted ?? "-"} />
            <p className="pt-2 text-[11px] leading-relaxed text-muted-foreground">
              倍率变动历史不会被这个按钮清理，避免丢失核心追踪记录。
            </p>
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
              <ScrollArea className="h-80">
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
    </section>
  )
}
