"use client"

import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import {
  AlertTriangle,
  CheckCircle2,
  CircleDollarSign,
  Clock3,
  Loader2,
  ExternalLink,
  LogIn,
  Pause,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  XCircle,
} from "lucide-react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { useConfirm } from "@/components/ui/confirm-dialog"
import { useChannels, useChannelRates } from "@/lib/queries"
import { apiFetch } from "@/lib/api"
import { useTriggerRefresh } from "@/lib/refresh-context"
import { channelTypeLabel, money, relativeTime, siteHostLabel } from "@/lib/format"
import { cn } from "@/lib/utils"
import { syncChannelStream, testLoginStream, type ProgressEvent } from "@/lib/sync-stream"
import type { Channel } from "@/lib/api-types"
import { ChannelFormDialog } from "@/components/monitor/channel-form-dialog"

type Status = "healthy" | "low" | "failed" | "idle"

function statusOf(c: Channel): Status {
  if (c.last_error) return "failed"
  if (c.last_balance == null) return "idle"
  if (c.balance_threshold > 0 && c.last_balance < c.balance_threshold) return "low"
  return "healthy"
}

const statusMap: Record<Status, { label: string; cls: string }> = {
  healthy: { label: "健康", cls: "text-success bg-success/10" },
  low: {
    label: "低余额",
    cls: "bg-amber-100 text-amber-700 ring-1 ring-inset ring-amber-300/80 dark:bg-amber-950/35 dark:text-amber-300 dark:ring-amber-400/40",
  },
  failed: { label: "登录失败", cls: "text-danger bg-danger/10" },
  idle: { label: "尚未采集", cls: "text-muted-foreground bg-muted/40" },
}

function StatusNotice({ channel, status }: { channel: Channel; status: Status }) {
  if (!channel.monitor_enabled) {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
        <Pause className="mt-0.5 size-3.5 shrink-0" />
        <span>监控已暂停，恢复后才会参与定时同步。</span>
      </div>
    )
  }
  if (status === "failed") {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-md border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger">
        <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
        <span>登录或采集失败，先点“测试登录”，通过后再同步。</span>
      </div>
    )
  }
  if (status === "low") {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-md border border-amber-300/80 bg-amber-50 px-3 py-2 text-xs text-amber-700 shadow-[inset_0_0_0_1px_rgba(245,158,11,0.08)] dark:border-amber-400/40 dark:bg-amber-950/25 dark:text-amber-300">
        <CircleDollarSign className="mt-0.5 size-3.5 shrink-0 text-amber-500" />
        <span>余额低于阈值，请补充余额或调整告警阈值。</span>
      </div>
    )
  }
  if (status === "idle") {
    return (
      <div className="mt-3 flex items-start gap-2 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
        <Clock3 className="mt-0.5 size-3.5 shrink-0" />
        <span>还没有采集结果，可以先手动同步一次。</span>
      </div>
    )
  }
  return null
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-xs font-medium text-foreground">{children}</span>
    </div>
  )
}

/** ratioTone 按倍率给 chip 上色，与 ChannelRatesPanel 共用同一套规则。 */
function ratioTone(r: number): string {
  if (r <= 0.8) return "bg-success/10 text-success ring-success/20"
  if (r > 2) return "bg-danger/10 text-danger ring-danger/20"
  if (r > 1.2) return "bg-warning/10 text-warning ring-warning/20"
  return "bg-muted text-foreground ring-border"
}

/** InlineRates 在渠道卡片内部展示当前分组倍率；过多时在固定区域内滚动，避免卡片高度跳动。 */
function InlineRates({ channelID }: { channelID: number }) {
  const { data, loading } = useChannelRates(channelID)
  const rates = [...(data ?? [])].sort((a, b) => a.ratio - b.ratio)

  if (loading) return null
  if (rates.length === 0) return null

  return (
    <div className="mt-3 border-t border-border pt-2.5">
      <div className="mb-1.5 flex items-center">
        <p className="text-[11px] text-muted-foreground">
          {rates.length} 个分组
        </p>
      </div>

      <div className="flex h-24 flex-wrap content-start gap-1 overflow-y-auto rounded-md border border-border/70 bg-muted/15 p-1.5 pr-1">
        {rates.map((r) => (
          <Tooltip key={r.id} delayDuration={150}>
            <TooltipTrigger asChild>
              <span
                className={cn(
                  "inline-flex cursor-default items-center gap-1 rounded px-1.5 py-0.5 text-[11px] ring-1 ring-inset transition-colors hover:bg-muted/60",
                  ratioTone(r.ratio),
                )}
              >
                <span className="font-medium">{r.model_name}</span>
                <span className="font-semibold tabular-nums">{r.ratio.toFixed(2)}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-xs text-xs">
              <p className="font-medium">{r.model_name}</p>
              {r.description ? (
                <p className="mt-0.5 text-muted-foreground">{r.description}</p>
              ) : (
                <p className="mt-0.5 italic text-muted-foreground">{"(无描述)"}</p>
              )}
              <p className="mt-0.5 text-muted-foreground">
                {"最近更新："}
                {relativeTime(r.last_seen_at)}
              </p>
            </TooltipContent>
          </Tooltip>
        ))}
      </div>
    </div>
  )
}

interface ChannelSyncState {
  running: boolean
  events: ProgressEvent[]
  latest: ProgressEvent | null
  finalOk: boolean | null
  fading: boolean
}

function emptySyncState(): ChannelSyncState {
  return { running: false, events: [], latest: null, finalOk: null, fading: false }
}

const stageLabel: Record<ProgressEvent["stage"], string> = {
  captcha: "打码",
  session: "会话",
  login: "登录",
  balance: "余额",
  rates: "倍率",
  done: "完成",
  error: "失败",
}

const stageOrder: Record<ProgressEvent["stage"], number> = {
  captcha: 1,
  session: 2,
  login: 3,
  balance: 4,
  rates: 5,
  done: 9,
  error: 9,
}

/** 按 stage 去重，每个 stage 只留最后一条事件（"在做中→完成"会被覆盖成完成态）。 */
function deriveSteps(events: ProgressEvent[]): ProgressEvent[] {
  const byStage = new Map<ProgressEvent["stage"], ProgressEvent>()
  for (const ev of events) byStage.set(ev.stage, ev)
  return [...byStage.values()].sort((a, b) => stageOrder[a.stage] - stageOrder[b.stage])
}

function SyncProgressStrip({ state }: { state: ChannelSyncState }) {
  if (!state.running && state.latest == null) return null
  const steps = deriveSteps(state.events)

  return (
    <div
      className={cn(
        "mt-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5",
        // 入场：上方滑入 + 淡入
        "animate-in fade-in slide-in-from-top-1 duration-300",
        // 出场：和 scheduleHide 里的 500ms 对齐
        "transition-all duration-500 ease-out",
        state.fading ? "-translate-y-0.5 opacity-0" : "opacity-100",
      )}
    >
      {steps.length === 0 ? (
        <div className="flex items-center gap-2 text-xs">
          <Loader2 className="size-3.5 shrink-0 animate-spin text-muted-foreground" />
          <span className="text-foreground/80">{"准备中…"}</span>
        </div>
      ) : (
        <ul className="space-y-1.5">
          {steps.map((ev) => {
            // 终止态：stage=done 或 error；显式 ok=true / false 也算
            const failed = ev.stage === "error" || ev.ok === false
            const succeeded = ev.stage === "done" || ev.ok === true
            const running = !failed && !succeeded
            const Icon = running ? Loader2 : failed ? XCircle : CheckCircle2
            const tone = running ? "text-muted-foreground" : failed ? "text-danger" : "text-success"
            return (
              <li
                key={ev.stage}
                className="flex items-center gap-2 text-xs animate-in fade-in duration-200"
              >
                <Icon
                  className={cn("size-3.5 shrink-0", tone, running && "animate-spin")}
                />
                <span className="w-9 shrink-0 text-[11px] text-muted-foreground">
                  {stageLabel[ev.stage]}
                </span>
                <span
                  className={cn(
                    "truncate",
                    failed ? "text-danger" : running ? "text-foreground/80" : "text-foreground",
                  )}
                >
                  {ev.message}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

export function ChannelCards() {
  const { data: channels, loading } = useChannels()
  const refresh = useTriggerRefresh()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [editing, setEditing] = useState<Channel | null>(null)
  const [creating, setCreating] = useState(false)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  // 每个渠道当前 sync 进度（最新一条事件） + 历史事件
  const [syncState, setSyncState] = useState<Record<number, ChannelSyncState>>({})

  // 成功后自动消失需要的两段定时器：先 5s 显示，再 500ms 过渡（与 strip 的 transition-opacity duration-500 对齐）。
  const hideTimers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())

  useEffect(() => {
    const timers = hideTimers.current
    return () => {
      timers.forEach((t) => clearTimeout(t))
      timers.clear()
    }
  }, [])

  function clearHideTimer(id: number) {
    const t = hideTimers.current.get(id)
    if (t != null) {
      clearTimeout(t)
      hideTimers.current.delete(id)
    }
  }

  function scheduleHide(id: number) {
    clearHideTimer(id)
    const t1 = setTimeout(() => {
      patchSync(id, (prev) => ({ ...prev, fading: true }))
      const t2 = setTimeout(() => {
        setSyncState((s) => {
          const { [id]: _gone, ...rest } = s
          void _gone
          return rest
        })
        hideTimers.current.delete(id)
      }, 500)
      hideTimers.current.set(id, t2)
    }, 5000)
    hideTimers.current.set(id, t1)
  }

  function patchSync(id: number, fn: (prev: ChannelSyncState) => ChannelSyncState) {
    setSyncState((s) => ({ ...s, [id]: fn(s[id] ?? emptySyncState()) }))
  }

  async function startStream(channel: Channel, action: "sync" | "test-login") {
    clearHideTimer(channel.id)
    patchSync(channel.id, () => ({
      running: true,
      events: [],
      latest: null,
      finalOk: null,
      fading: false,
    }))
    let sawError = false
    const stream = action === "sync" ? syncChannelStream : testLoginStream
    try {
      await stream(channel.id, {
        onEvent: (ev) => {
          if (ev.stage === "error" || ev.ok === false) sawError = true
          patchSync(channel.id, (prev) => ({
            ...prev,
            events: [...prev.events, ev],
            latest: ev,
          }))
        },
      })
      const ok = !sawError
      patchSync(channel.id, (prev) => ({
        ...prev,
        running: false,
        finalOk: ok,
      }))
      if (ok) scheduleHide(channel.id)
    } catch (e) {
      const err = e as Error
      const failureLabel = action === "sync" ? "同步失败" : "测试登录失败"
      patchSync(channel.id, (prev) => ({
        ...prev,
        running: false,
        finalOk: false,
        latest: {
          stage: "error",
          message: err.message || failureLabel,
          time: new Date().toISOString(),
        },
      }))
      // 失败保留，不调度自动隐藏
    } finally {
      refresh()
    }
  }

  async function withBusy(key: string, fn: () => Promise<unknown>) {
    setBusyAction(key)
    try {
      await fn()
      refresh()
    } catch (e) {
      const err = e as Error
      toast.error(err.message || "操作失败")
    } finally {
      setBusyAction(null)
    }
  }

  return (
    <section>
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-baseline gap-3">
          <h2 className="text-base font-semibold text-foreground">{"渠道"}</h2>
          <p className="text-xs text-muted-foreground">{"实时健康、余额与同步状态"}</p>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-muted-foreground">
            {channels?.length ?? 0}{" 个渠道"}
          </span>
          <Button
            size="sm"
            className="gap-1.5 text-xs"
            onClick={() => {
              setEditing(null)
              setCreating(true)
            }}
          >
            <Plus className="size-3.5" />
            {"新增"}
          </Button>
        </div>
      </div>

      {loading ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
          {"加载中…"}
        </p>
      ) : !channels || channels.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border px-4 py-10 text-center">
          <p className="text-sm text-muted-foreground">{"还没有任何渠道。"}</p>
          <Button
            size="sm"
            className="mt-3 gap-1.5"
            onClick={() => {
              setEditing(null)
              setCreating(true)
            }}
          >
            <Plus className="size-3.5" />
            {"添加第一个渠道"}
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
          {channels.map((c) => {
            const status = statusOf(c)
            const meta = statusMap[status]
            const siteLabel = siteHostLabel(c.site_url)
            return (
              <Card
                key={c.id}
                id={`channel-${c.id}`}
                className={cn(
                  "group/channel flex h-full flex-col gap-0 border border-border p-4 shadow-none transition-[box-shadow,border-color]",
                  status === "failed" && "border-danger/30",
                  status === "low" &&
                    "border-amber-300/90 bg-amber-50/30 shadow-[0_0_0_1px_rgba(245,158,11,0.16),0_18px_38px_-30px_rgba(245,158,11,0.72)] dark:border-amber-400/45 dark:bg-amber-950/10",
                )}
              >
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <span className="min-w-0 truncate text-sm font-semibold text-foreground">{c.name}</span>
                  <span
                    className={cn(
                      "inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ring-1 ring-inset",
                      c.type === "newapi"
                        ? "bg-brand/10 text-brand ring-brand/20"
                        : "bg-foreground/5 text-foreground ring-border",
                    )}
                  >
                    {channelTypeLabel(c.type)}
                  </span>
                  {siteLabel ? (
                    <a
                      href={c.site_url}
                      target="_blank"
                      rel="noreferrer"
                      title="打开原站点"
                      aria-label="打开原站点"
                      className="inline-flex max-w-6 items-center gap-1 overflow-hidden whitespace-nowrap rounded bg-cyan-50 px-1.5 py-0.5 text-[10px] font-semibold text-cyan-700 ring-1 ring-inset ring-cyan-200/80 transition-[max-width,background-color,color,box-shadow] duration-200 hover:max-w-[180px] hover:bg-cyan-100 hover:text-cyan-800 hover:shadow-[0_0_0_2px_rgba(6,182,212,0.12)] focus:max-w-[180px] focus:bg-cyan-100 focus:text-cyan-800 focus:outline-none focus:ring-cyan-300 group-hover/channel:max-w-[180px] group-focus-within/channel:max-w-[180px] dark:bg-cyan-950/30 dark:text-cyan-300 dark:ring-cyan-500/30 dark:hover:bg-cyan-900/40"
                    >
                      <ExternalLink className="size-3 shrink-0" />
                      <span className="truncate opacity-0 transition-opacity duration-150 group-hover/channel:opacity-100 group-focus-within/channel:opacity-100">
                        {siteLabel}
                      </span>
                    </a>
                  ) : null}
                  {!c.monitor_enabled ? (
                    <span className="inline-flex items-center rounded bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      {"已暂停"}
                    </span>
                  ) : null}
                </div>

                <div className="mt-3 divide-y divide-border">
                  <Row label="余额">{money(c.last_balance)}</Row>
                  <Row label="阈值">{c.balance_threshold > 0 ? money(c.balance_threshold) : "未设置"}</Row>
                  <Row label="状态">
                    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium", meta.cls)}>
                      {meta.label}
                    </span>
                  </Row>
                  <Row label="健康分">
                    <span
                      className={cn(
                        "tabular-nums",
                        (c.health_score ?? 0) >= 80
                          ? "text-success"
                          : (c.health_score ?? 0) >= 60
                            ? "text-warning"
                            : "text-danger",
                      )}
                    >
                      {c.health_score ?? "—"}
                    </span>
                  </Row>
                  <Row label="上次更新">{relativeTime(c.last_balance_at ?? c.updated_at)}</Row>
                  {c.last_error ? (
                    <div className="py-1">
                      <p className="break-all text-[11px] text-danger" title={c.last_error}>
                        {c.last_error.length > 80 ? c.last_error.slice(0, 80) + "…" : c.last_error}
                      </p>
                    </div>
                  ) : null}
                </div>

                <StatusNotice channel={c} status={status} />

                <InlineRates channelID={c.id} />

                <div className="mt-3 grid grid-cols-3 gap-2">
                  <Button
                    variant={status === "idle" || status === "low" ? "default" : "outline"}
                    size="sm"
                    className="gap-1 text-xs"
                    disabled={!!syncState[c.id]?.running}
                    onClick={() => startStream(c, "sync")}
                  >
                    <RefreshCw
                      className={cn("size-3", syncState[c.id]?.running && "animate-spin")}
                    />
                    {"同步"}
                  </Button>
                  <Button
                    variant={status === "failed" ? "default" : "outline"}
                    size="sm"
                    className="gap-1 text-xs"
                    disabled={!!syncState[c.id]?.running}
                    onClick={() => startStream(c, "test-login")}
                  >
                    <LogIn className="size-3" />
                    {"测试登录"}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1 text-xs"
                    onClick={() => {
                      setEditing(c)
                      setCreating(true)
                    }}
                  >
                    <Pencil className="size-3" />
                    {"编辑"}
                  </Button>
                </div>

                <SyncProgressStrip state={syncState[c.id] ?? emptySyncState()} />

                <div className="mt-auto pt-3">
                  <div className="flex items-center justify-between gap-2 border-t border-border pt-2.5">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="gap-1 text-xs text-muted-foreground"
                      disabled={busyAction === `toggle-${c.id}`}
                      onClick={() =>
                        withBusy(`toggle-${c.id}`, () =>
                          apiFetch(`/channels/${c.id}/${c.monitor_enabled ? "disable" : "enable"}`, {
                            method: "POST",
                          }),
                        )
                      }
                    >
                      {c.monitor_enabled ? (
                        <Pause className="size-3" />
                      ) : (
                        <Play className="size-3" />
                      )}
                      {c.monitor_enabled ? "暂停监控" : "恢复监控"}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="gap-1 text-xs text-destructive hover:bg-destructive/10 hover:text-destructive"
                      disabled={busyAction === `delete-${c.id}`}
                      onClick={async () => {
                        const ok = await confirm({
                          title: `删除渠道 ${c.name}？`,
                          description: "删除后该渠道的余额历史、倍率快照与登录凭据都将一并清除，且无法恢复。",
                          confirmLabel: "删除",
                          destructive: true,
                        })
                        if (!ok) return
                        void withBusy(`delete-${c.id}`, () =>
                          apiFetch(`/channels/${c.id}`, { method: "DELETE" }),
                        )
                      }}
                    >
                      <Trash2 className="size-3" />
                      {"删除"}
                    </Button>
                  </div>
                </div>
              </Card>
            )
          })}
        </div>
      )}

      <ChannelFormDialog
        open={creating}
        onOpenChange={(v) => {
          setCreating(v)
          if (!v) setEditing(null)
        }}
        channel={editing}
      />

      {confirmDialog}
    </section>
  )
}
