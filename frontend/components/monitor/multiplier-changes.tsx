"use client"

import { useMemo, useState } from "react"
import { ArrowDownRight, ArrowUpRight, LocateFixed } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useDashboardSummary, useChannels } from "@/lib/queries"
import { channelTypeLabel, ratioDelta, relativeTime, shortTime } from "@/lib/format"
import { cn } from "@/lib/utils"

type DirectionFilter = "all" | "up" | "down"

function scrollToChannel(channelID: number) {
  const el = document.getElementById(`channel-${channelID}`)
  if (!el) return
  el.scrollIntoView({ behavior: "smooth", block: "center" })
  el.classList.add("ring-2", "ring-brand/60")
  window.setTimeout(() => {
    el.classList.remove("ring-2", "ring-brand/60")
  }, 1600)
}

export function MultiplierChanges() {
  const summary = useDashboardSummary()
  const channels = useChannels()
  const [direction, setDirection] = useState<DirectionFilter>("all")
  const [channelFilter, setChannelFilter] = useState("all")

  const channelMap = useMemo(() => {
    const m = new Map<number, { name: string; type: string }>()
    for (const c of channels.data ?? []) m.set(c.id, { name: c.name, type: c.type })
    return m
  }, [channels.data])

  const items = summary.data?.recent_rate_changes ?? []
  const filteredItems = useMemo(
    () =>
      items.filter((m) => {
        const delta = ratioDelta(m.old_ratio, m.new_ratio)
        if (direction !== "all" && delta.direction !== direction) return false
        if (channelFilter !== "all" && m.channel_id !== Number(channelFilter)) return false
        return true
      }),
    [channelFilter, direction, items],
  )
  const upCount = items.filter((m) => ratioDelta(m.old_ratio, m.new_ratio).direction === "up").length
  const downCount = items.length - upCount

  return (
    <Card className="max-h-100 min-h-0 overflow-hidden border border-border shadow-none lg:h-100">
      <CardHeader className="flex shrink-0 flex-col gap-3 pb-2">
        <div className="flex items-center justify-between gap-3">
          <div>
            <CardTitle className="text-base font-semibold">{"最近倍率变动"}</CardTitle>
            <p className="mt-1 text-xs text-muted-foreground">
              {items.length > 0 ? `${upCount} 个上涨 · ${downCount} 个下降` : "暂无变化"}
            </p>
          </div>
          <span className="text-xs text-muted-foreground">{filteredItems.length > 0 ? `${filteredItems.length} 条` : ""}</span>
        </div>
        <div className="grid gap-2 sm:grid-cols-[1fr_150px]">
          <ToggleGroup
            type="single"
            value={direction}
            onValueChange={(value) => {
              if (value) setDirection(value as DirectionFilter)
            }}
            variant="outline"
            size="sm"
            className="grid w-full grid-cols-3"
            aria-label="倍率变化方向筛选"
          >
            <ToggleGroupItem value="all" className="text-xs">全部</ToggleGroupItem>
            <ToggleGroupItem value="up" className="text-xs">上涨</ToggleGroupItem>
            <ToggleGroupItem value="down" className="text-xs">下降</ToggleGroupItem>
          </ToggleGroup>
          <Select value={channelFilter} onValueChange={setChannelFilter}>
            <SelectTrigger size="sm" className="w-full text-xs">
              <SelectValue placeholder="全部渠道" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部渠道</SelectItem>
              {(channels.data ?? []).map((c) => (
                <SelectItem key={c.id} value={String(c.id)}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </CardHeader>
      <CardContent className="min-h-0 flex-1 px-0">
        {summary.loading ? (
          <p className="px-6 py-6 text-xs text-muted-foreground">{"加载中…"}</p>
        ) : items.length === 0 ? (
          <p className="px-6 py-6 text-xs text-muted-foreground">{"暂无倍率变动记录"}</p>
        ) : filteredItems.length === 0 ? (
          <div className="px-6 py-6">
            <p className="text-xs text-muted-foreground">{"当前筛选下没有倍率变动"}</p>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="mt-2 h-7 px-2 text-xs"
              onClick={() => {
                setDirection("all")
                setChannelFilter("all")
              }}
            >
              清除筛选
            </Button>
          </div>
        ) : (
          <ScrollArea type="hover" className="h-full">
            <ul className="divide-y divide-border">
              {filteredItems.map((m) => {
                const ch = channelMap.get(m.channel_id)
                const delta = ratioDelta(m.old_ratio, m.new_ratio)
                const isUp = delta.direction === "up"
                const chType = ch?.type ?? ""
                return (
                  <li key={m.id} className="flex items-start gap-3 px-6 py-3.5">
                    <div className="flex flex-col items-center gap-0.5 pt-1">
                      <span className={cn("size-2 rounded-full", isUp ? "bg-danger" : "bg-success")} />
                    </div>
                    <div className="shrink-0 text-xs text-muted-foreground leading-relaxed">
                      <p>{relativeTime(m.changed_at)}</p>
                      <p>{shortTime(m.changed_at)}</p>
                    </div>

                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold text-foreground">{m.model_name}</span>
                        <span
                          className={cn(
                            "inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium ring-1 ring-inset",
                            chType === "newapi"
                              ? "bg-brand/10 text-brand ring-brand/20"
                              : "bg-foreground/5 text-foreground ring-border",
                          )}
                        >
                          {ch?.name ?? `#${m.channel_id}`}
                          {chType ? <span className="ml-1 opacity-60">{channelTypeLabel(chType)}</span> : null}
                        </span>
                      </div>
                      <div className="mt-1.5 flex items-center text-xs">
                        <div>
                          <span className="text-muted-foreground">{"倍率"}</span>
                          <p className="mt-0.5 tabular-nums">
                            <span className="text-muted-foreground">
                              {m.old_ratio == null ? "—" : m.old_ratio.toFixed(2)}
                            </span>
                            <span className="mx-1 text-muted-foreground">{"→"}</span>
                            <span className={cn("font-medium", isUp ? "text-danger" : "text-success")}>
                              {m.new_ratio.toFixed(2)}
                            </span>
                          </p>
                        </div>
                      </div>
                    </div>

                    <div className="flex shrink-0 flex-col items-end gap-1 pt-0.5">
                      <span
                        className={cn(
                          "inline-flex items-center gap-0.5 rounded-md px-2 py-1 text-xs font-semibold",
                          isUp ? "bg-danger/10 text-danger" : "bg-success/10 text-success",
                        )}
                      >
                        {isUp ? <ArrowUpRight className="size-3" /> : <ArrowDownRight className="size-3" />}
                        {delta.pct}
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7 text-muted-foreground hover:text-foreground"
                        title="定位到渠道"
                        onClick={() => scrollToChannel(m.channel_id)}
                      >
                        <LocateFixed className="size-3.5" />
                      </Button>
                    </div>
                  </li>
                )
              })}
            </ul>
          </ScrollArea>
        )}
      </CardContent>
    </Card>
  )
}
