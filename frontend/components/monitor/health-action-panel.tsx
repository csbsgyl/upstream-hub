"use client"

import { useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import {
  Activity,
  Bell,
  CheckCircle2,
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Settings,
  ShieldAlert,
} from "lucide-react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { useAddChannel } from "@/lib/add-channel-context"
import { apiFetch } from "@/lib/api"
import { useTriggerRefresh } from "@/lib/refresh-context"
import {
  useCaptchaConfigs,
  useChannels,
  useNotificationChannels,
  useOpsStatus,
} from "@/lib/queries"
import { cn } from "@/lib/utils"
import type { Channel, OpsScanResult } from "@/lib/api-types"

type Tone = "success" | "warning" | "danger" | "muted" | "brand"

function toneClass(tone: Tone) {
  switch (tone) {
    case "success":
      return "bg-success/10 text-success ring-success/20"
    case "warning":
      return "bg-warning/10 text-warning ring-warning/20"
    case "danger":
      return "bg-danger/10 text-danger ring-danger/20"
    case "brand":
      return "bg-brand/10 text-brand ring-brand/20"
    default:
      return "bg-muted text-muted-foreground ring-border"
  }
}

function Metric({
  label,
  value,
  tone,
}: {
  label: string
  value: string | number
  tone: Tone
}) {
  return (
    <div className={cn("inline-flex items-center gap-2 rounded-md px-2.5 py-1.5 text-xs ring-1", toneClass(tone))}>
      <span className="font-medium">{label}</span>
      <span className="font-semibold tabular-nums">{value}</span>
    </div>
  )
}

function channelBuckets(channels: Channel[]) {
  return channels.reduce(
    (acc, c) => {
      if (!c.monitor_enabled) {
        acc.paused++
      } else if (c.last_error) {
        acc.failed++
      } else if (c.last_balance == null) {
        acc.idle++
      } else if (c.balance_threshold > 0 && c.last_balance < c.balance_threshold) {
        acc.low++
      } else {
        acc.healthy++
      }
      return acc
    },
    { healthy: 0, low: 0, failed: 0, idle: 0, paused: 0 },
  )
}

export function HealthActionPanel() {
  const channels = useChannels()
  const notifications = useNotificationChannels()
  const captchas = useCaptchaConfigs()
  const ops = useOpsStatus()
  const refresh = useTriggerRefresh()
  const { openAdd } = useAddChannel()
  const [syncingAll, setSyncingAll] = useState(false)

  const list = channels.data ?? []
  const buckets = useMemo(() => channelBuckets(list), [list])
  const enabledNotifications = (notifications.data ?? []).filter((n) => n.enabled).length
  const enabledCaptchas = (captchas.data ?? []).filter((c) => c.enabled).length
  const totalProblemCount = buckets.failed + buckets.low + buckets.idle
  const systemReady = ops.data?.database === "ok" && Boolean(ops.data?.app_secret_ready)
  const loading = (channels.loading && list.length === 0) || (ops.loading && !ops.data)

  async function syncAll() {
    setSyncingAll(true)
    try {
      const res = await apiFetch<OpsScanResult>("/ops/scan/sync", { method: "POST" })
      if (!res.started) {
        toast.warning(res.message || "同步任务没有启动")
        return
      }
      toast.success(res.message || "已开始同步余额和倍率")
      refresh()
    } catch (e) {
      const err = e as Error
      toast.error(err.message || "启动同步失败")
    } finally {
      setSyncingAll(false)
    }
  }

  return (
    <Card className="border border-border p-3 shadow-none">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="flex min-w-0 flex-1 flex-col gap-3 lg:flex-row lg:items-center">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "flex size-9 shrink-0 items-center justify-center rounded-md ring-1",
                loading
                  ? toneClass("muted")
                  : totalProblemCount > 0
                    ? toneClass("warning")
                    : toneClass(systemReady ? "success" : "danger"),
              )}
            >
              {loading ? (
                <Loader2 className="size-4 animate-spin" />
              ) : totalProblemCount > 0 || !systemReady ? (
                <ShieldAlert className="size-4" />
              ) : (
                <CheckCircle2 className="size-4" />
              )}
            </span>
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-foreground">
                {loading
                  ? "正在读取运行状态"
                  : totalProblemCount > 0
                    ? "有项目需要处理"
                    : systemReady
                      ? "运行状态正常"
                      : "系统配置需要检查"}
              </p>
              <p className="text-xs text-muted-foreground">
                同步任务会同时更新余额和倍率
              </p>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Metric label="健康" value={buckets.healthy} tone="success" />
            <Metric label="低余额" value={buckets.low} tone={buckets.low > 0 ? "warning" : "muted"} />
            <Metric label="失败" value={buckets.failed} tone={buckets.failed > 0 ? "danger" : "muted"} />
            <Metric label="未采集" value={buckets.idle} tone={buckets.idle > 0 ? "warning" : "muted"} />
            <Metric label="暂停" value={buckets.paused} tone={buckets.paused > 0 ? "muted" : "success"} />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" className="gap-1.5 text-xs" disabled={syncingAll || list.length === 0} onClick={syncAll}>
            <RefreshCw className={cn("size-3.5", syncingAll && "animate-spin")} />
            同步全部
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 text-xs" onClick={openAdd}>
            <Plus className="size-3.5" />
            新增渠道
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 text-xs" asChild>
            <Link to="/notifications">
              <Bell className="size-3.5" />
              通知 {enabledNotifications}
            </Link>
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 text-xs" asChild>
            <Link to="/captcha">
              <KeyRound className="size-3.5" />
              打码 {enabledCaptchas}
            </Link>
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 text-xs" asChild>
            <Link to="/settings">
              {systemReady ? <Activity className="size-3.5" /> : <Settings className="size-3.5" />}
              运维
            </Link>
          </Button>
        </div>
      </div>
    </Card>
  )
}
