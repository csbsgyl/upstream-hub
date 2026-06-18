"use client"

import { useMemo, useState } from "react"
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis, CartesianGrid } from "recharts"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useBalanceTrend, useDashboardSummary } from "@/lib/queries"
import { money } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { BalanceTrendRange } from "@/lib/api-types"

function formatY(n: number) {
  if (n === 0) return "$0"
  if (n >= 1000) return `$${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}K`
  if (n >= 100) return `$${n.toFixed(0)}`
  return `$${n.toFixed(n >= 10 ? 1 : 2)}`
}

/**
 * niceCeil 把最大值向上取整到一个"好看的"刻度，避免曲线贴顶。
 * 例如 47 → 50；478 → 500；12,300 → 15,000。
 */
function niceCeil(n: number): number {
  if (!Number.isFinite(n) || n <= 0) return 10
  const padded = n * 1.15
  const mag = Math.pow(10, Math.floor(Math.log10(padded)))
  const norm = padded / mag
  const step = norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10
  return step * mag
}

const RANGE_OPTIONS: Array<{ value: BalanceTrendRange; label: string; meta: string }> = [
  { value: "24h", label: "24小时", meta: "5分钟采样" },
  { value: "7d", label: "7天", meta: "小时聚合" },
  { value: "30d", label: "1个月", meta: "日聚合" },
]

function formatDate(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

function formatTick(iso: string, range: BalanceTrendRange) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  if (range === "24h") {
    return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false })
  }
  if (range === "7d") {
    return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:00`
  }
  return formatDate(iso)
}

function formatTooltipTime(iso: string, range: BalanceTrendRange) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  if (range === "30d") return formatDate(iso)
  return `${formatDate(iso)} ${d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false })}`
}

interface ChartPoint {
  at: string
  tooltipLabel: string
  balance: number
}

interface TooltipPayloadItem {
  value: number
  payload?: ChartPoint
}

function ChartTooltip({ active, payload, label }: { active?: boolean; payload?: TooltipPayloadItem[]; label?: string }) {
  if (!active || !payload?.length) return null
  const point = payload[0].payload
  return (
    <div className="rounded-lg border border-border bg-popover px-3 py-2 shadow-md">
      <p className="text-xs text-muted-foreground">{point?.tooltipLabel ?? label}</p>
      <p className="text-sm font-semibold text-foreground">
        {"$"}{payload[0].value.toLocaleString("en-US")}
      </p>
    </div>
  )
}

export function BalanceOverview() {
  const [range, setRange] = useState<BalanceTrendRange>("24h")
  const trend = useBalanceTrend(range)
  const summary = useDashboardSummary()
  const activeRange = RANGE_OPTIONS.find((item) => item.value === range) ?? RANGE_OPTIONS[0]

  const data = useMemo(
    () =>
      (trend.data ?? []).map((p) => {
        const at = p.at ?? p.day ?? ""
        return {
          at,
          tooltipLabel: formatTooltipTime(at, range),
          balance: p.balance,
        }
      }),
    [trend.data, range],
  )

  const channels = summary.data?.channels ?? []
  const yMax = data.length > 0 ? niceCeil(Math.max(...data.map((d) => d.balance))) : 10
  const showDots = data.length <= 40
  const latestPoint = data.at(-1)
  const highestPoint = data.length > 0 ? data.reduce((max, d) => (d.balance > max.balance ? d : max), data[0]) : null
  const lowestPoint = data.length > 0 ? data.reduce((min, d) => (d.balance < min.balance ? d : min), data[0]) : null

  return (
    <Card className="border border-border shadow-none lg:h-100">
      <CardHeader className="flex shrink-0 flex-col gap-3 pb-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <CardTitle className="text-base font-semibold">{"余额概览"}</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">{`最近 ${activeRange.label} · ${activeRange.meta}`}</p>
        </div>
        <ToggleGroup
          type="single"
          value={range}
          onValueChange={(value) => {
            if (value) setRange(value as BalanceTrendRange)
          }}
          variant="outline"
          size="sm"
          className="grid w-full grid-cols-3 sm:w-auto"
          aria-label="余额趋势时间范围"
        >
          {RANGE_OPTIONS.map((item) => (
            <ToggleGroupItem key={item.value} value={item.value} className="min-w-16 px-2 text-xs">
              {item.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col">
        {data.length > 0 ? (
          <div className="mb-3 grid grid-cols-2 gap-2 md:grid-cols-4">
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2">
              <p className="text-[11px] text-muted-foreground">当前</p>
              <p className="mt-1 text-sm font-semibold tabular-nums text-foreground">{money(latestPoint?.balance)}</p>
            </div>
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2">
              <p className="text-[11px] text-muted-foreground">最高</p>
              <p className="mt-1 text-sm font-semibold tabular-nums text-success">{money(highestPoint?.balance)}</p>
            </div>
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2">
              <p className="text-[11px] text-muted-foreground">最低</p>
              <p className="mt-1 text-sm font-semibold tabular-nums text-warning">{money(lowestPoint?.balance)}</p>
            </div>
            <div className="rounded-md border border-border bg-muted/20 px-3 py-2">
              <p className="text-[11px] text-muted-foreground">采样点</p>
              <p className="mt-1 text-sm font-semibold tabular-nums text-foreground">{data.length}</p>
            </div>
          </div>
        ) : null}
        <div className="h-56 min-h-56 w-full lg:min-h-0 lg:flex-1">
          {trend.loading ? (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">{"加载中…"}</div>
          ) : data.length === 0 ? (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
              {"暂无余额采样，等待下次扫描或手动刷新"}
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                <XAxis
                  dataKey="at"
                  tickLine={false}
                  axisLine={false}
                  tick={{ fill: "var(--muted-foreground)", fontSize: 11 }}
                  tickFormatter={(value) => formatTick(String(value), range)}
                  dy={8}
                  minTickGap={range === "24h" ? 28 : 20}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  width={48}
                  tick={{ fill: "var(--muted-foreground)", fontSize: 11 }}
                  tickFormatter={formatY}
                  domain={[0, yMax]}
                />
                <Tooltip content={<ChartTooltip />} cursor={{ stroke: "var(--border)", strokeDasharray: "4 4" }} />
                <Line
                  type="monotone"
                  dataKey="balance"
                  stroke="var(--brand)"
                  strokeWidth={2}
                  dot={showDots ? { r: 4, fill: "var(--background)", stroke: "var(--brand)", strokeWidth: 2 } : false}
                  activeDot={{ r: 5, fill: "var(--brand)", strokeWidth: 0 }}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* per-channel chips */}
        {channels.length > 0 ? (
          <div className="mt-3 flex shrink-0 flex-wrap items-center gap-x-5 gap-y-2 border-t border-border pt-3">
            {channels.map((c) => {
              const isFailed = !!c.last_error
              const isUnknown = c.last_balance == null
              return (
                <span key={c.id} className="inline-flex items-center gap-1.5 text-xs">
                  <span
                    className={cn(
                      "size-2 rounded-full",
                      isFailed ? "bg-danger" : isUnknown ? "bg-muted-foreground/40" : "bg-success",
                    )}
                  />
                  <span className="font-medium text-foreground">{c.name}</span>
                  <span className="tabular-nums text-muted-foreground">{money(c.last_balance)}</span>
                </span>
              )
            })}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
