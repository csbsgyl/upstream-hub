"use client"

import { useState } from "react"
import { toast } from "sonner"
import {
  Activity,
  BellRing,
  DatabaseBackup,
  Download,
  FileJson,
  RotateCcw,
  ShieldCheck,
} from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { apiFetch } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import { useAuditLogs, useFailedNotificationLogs, useOpsStatus } from "@/lib/queries"
import { useTriggerRefresh } from "@/lib/refresh-context"
import { relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"

function asText(v: unknown, fallback = "—") {
  if (v == null || v === "") return fallback
  if (Array.isArray(v)) return v.length === 0 ? "—" : v.join(", ")
  if (typeof v === "boolean") return v ? "开启" : "关闭"
  return String(v)
}

function boolTone(ok: boolean) {
  return ok ? "bg-success/10 text-success ring-success/20" : "bg-danger/10 text-danger ring-danger/20"
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-medium text-foreground">{value}</span>
    </div>
  )
}

export default function SettingsPage() {
  const { username } = useAuth()
  const status = useOpsStatus()
  const audits = useAuditLogs(80)
  const failedNotifications = useFailedNotificationLogs(50)
  const refresh = useTriggerRefresh()
  const [busyRetry, setBusyRetry] = useState<number | null>(null)
  const [downloading, setDownloading] = useState(false)

  const s = status.data
  const latestBackup = s?.backups?.[0] ?? null
  const failed = failedNotifications.data ?? []

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
      refresh()
      failedNotifications.refetch()
      audits.refetch()
    } catch (e) {
      const err = e as Error
      toast.error(err.message || "重发失败")
    } finally {
      setBusyRetry(null)
    }
  }

  async function downloadDiagnostics() {
    setDownloading(true)
    try {
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
    } catch (e) {
      const err = e as Error
      toast.error(err.message || "诊断包下载失败")
    } finally {
      setDownloading(false)
    }
  }

  return (
    <section className="space-y-3">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold text-foreground">{"运维中心"}</h1>
          <p className="text-xs text-muted-foreground">
            {"自检、备份、通知补发、审计日志和诊断导出。"}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="gap-1.5"
          disabled={downloading}
          onClick={downloadDiagnostics}
        >
          <Download className="size-3.5" />
          {"下载诊断包"}
        </Button>
      </header>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <ShieldCheck className="size-4 text-success" />
              {"系统自检"}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="数据库" value={<Badge className={cn("ring-1", boolTone(s?.database === "ok"))}>{s?.database ?? "检查中"}</Badge>} />
            <Row label="鉴权" value={<Badge variant="outline">{s ? (s.auth_enabled ? "已开启" : "未开启") : "—"}</Badge>} />
            <Row label="APP_SECRET" value={<Badge className={cn("ring-1", boolTone(Boolean(s?.app_secret_ready)))}>{s?.app_secret_ready ? "有效" : "缺失"}</Badge>} />
            <Row label="当前账号" value={username ?? "—"} />
            <Row label="状态生成" value={relativeTime(s?.generated_at)} />
          </CardContent>
        </Card>

        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <Activity className="size-4 text-brand" />
              {"调度与倍率通知"}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="余额扫描" value={asText(s?.scheduler?.balance_cron)} />
            <Row label="倍率扫描" value={asText(s?.scheduler?.rate_cron)} />
            <Row label="倍率合并" value={asText(s?.notifications?.batch_rate_changes)} />
            <Row label="最小变化" value={`${asText(s?.notifications?.min_change_pct, "0")}%`} />
            <Row label="方向过滤" value={asText(s?.notifications?.rate_change_direction, "all")} />
            <Row label="静默分组" value={asText(s?.notifications?.rate_change_quiet_groups)} />
          </CardContent>
        </Card>

        <Card className="border border-border shadow-none">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <DatabaseBackup className="size-4 text-warning" />
              {"备份状态"}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <Row label="最新备份" value={latestBackup ? latestBackup.name : "未发现"} />
            <Row label="备份时间" value={latestBackup ? relativeTime(latestBackup.updated_at) : "—"} />
            <Row label="备份文件数" value={s?.backups?.length ?? 0} />
            <p className="pt-2 text-[11px] leading-relaxed text-muted-foreground">
              {"服务器执行 ./scripts/backup.sh 可手动备份；恢复需 ./scripts/restore.sh <sql.gz> --yes。"}
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Card className="border border-border shadow-none">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="flex items-center gap-2 text-base font-semibold">
              <BellRing className="size-4 text-danger" />
              {"失败通知补发"}
            </CardTitle>
            <Badge variant="outline">{failed.length}</Badge>
          </CardHeader>
          <CardContent className="px-0">
            {failedNotifications.loading ? (
              <p className="px-6 py-4 text-xs text-muted-foreground">{"加载中…"}</p>
            ) : failed.length === 0 ? (
              <p className="px-6 py-4 text-xs text-muted-foreground">{"暂无失败通知。"}</p>
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
                        {"重发"}
                      </Button>
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
              {"操作审计"}
            </CardTitle>
            <Badge variant="outline">{audits.data?.length ?? 0}</Badge>
          </CardHeader>
          <CardContent className="px-0">
            {audits.loading ? (
              <p className="px-6 py-4 text-xs text-muted-foreground">{"加载中…"}</p>
            ) : !audits.data || audits.data.length === 0 ? (
              <p className="px-6 py-4 text-xs text-muted-foreground">{"暂无审计记录。"}</p>
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
