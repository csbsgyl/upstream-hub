import { useEffect, useMemo, useState } from "react"
import { useTheme } from "next-themes"
import { Github, LogOut, RefreshCw, Sun, Moon, Settings, Sparkles } from "lucide-react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { useAuth } from "@/lib/auth-context"
import { useTriggerRefresh } from "@/lib/refresh-context"
import { useChannels, useVersionCheck } from "@/lib/queries"
import { relativeTime } from "@/lib/format"

export function MonitorHeader() {
  const { theme, setTheme } = useTheme()
  const { username, authDisabled, logout } = useAuth()
  const refresh = useTriggerRefresh()
  const channels = useChannels()
  const version = useVersionCheck()
  const [mounted, setMounted] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const versionInfo = version.data
  const hasUpdate = Boolean(versionInfo?.has_update && !versionInfo.check_error)
  const repoURL = versionInfo?.current.repo_url ?? "https://github.com/csbsgyl/upstream-hub"
  const repoName = versionInfo?.current.repository ?? "csbsgyl/upstream-hub"

  useEffect(() => setMounted(true), [])

  /**
   * 找出所有渠道中最近一次采集时间——这是"上次采集"展示的依据，
   * 让用户知道页面上的余额到底是多新的快照（区别于"我刚点了刷新"）。
   */
  const lastCollectedAt = useMemo(() => {
    const list = channels.data ?? []
    let best: string | null = null
    let bestT = -Infinity
    for (const c of list) {
      if (!c.last_balance_at) continue
      const t = new Date(c.last_balance_at).getTime()
      if (Number.isFinite(t) && t > bestT) {
        bestT = t
        best = c.last_balance_at
      }
    }
    return best
  }, [channels.data])

  function handleRefresh() {
    setSyncing(true)
    refresh()
    setTimeout(() => setSyncing(false), 800)
  }

  return (
    <header className="sticky top-0 z-20 border-b border-border bg-background/95 backdrop-blur-sm">
      <div className="mx-auto flex h-14 max-w-360 items-center justify-between gap-4 px-5">
        {/* left: logo + title */}
        <Link
          to="/"
          className="flex min-w-0 items-center gap-3 rounded-md outline-none transition-opacity hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          aria-label="返回首页"
        >
          <div className="relative flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-slate-950 shadow-sm ring-1 ring-border">
            <svg
              viewBox="0 0 32 32"
              className="size-6"
              aria-hidden="true"
            >
              <path
                d="M7 11.5h9.5c4.2 0 7.5 3.1 7.5 7.2v1.6"
                fill="none"
                stroke="#22d3ee"
                strokeWidth="3"
                strokeLinecap="round"
              />
              <path
                d="M7 20.5h9.5c4.2 0 7.5-3.1 7.5-7.2v-1.6"
                fill="none"
                stroke="#7dd3fc"
                strokeWidth="3"
                strokeLinecap="round"
              />
              <path
                d="M20.5 8.5 25 12.5 20.5 16.5"
                fill="none"
                stroke="#f8fafc"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <circle cx="7" cy="11.5" r="2.1" fill="#22c55e" />
              <circle cx="7" cy="20.5" r="2.1" fill="#a3e635" />
              <circle cx="24" cy="16" r="2.3" fill="#f8fafc" />
            </svg>
          </div>
          <div className="min-w-0 leading-tight">
            <div className="flex items-baseline gap-2">
              <h1 className="truncate text-base font-semibold tracking-tight text-foreground">
                {"上游监控台"}
              </h1>
              <span className="hidden rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-700 ring-1 ring-emerald-200 sm:inline-flex dark:bg-emerald-950/30 dark:text-emerald-300 dark:ring-emerald-800">
                {"Live"}
              </span>
            </div>
            <p className="hidden text-[11px] font-medium text-muted-foreground sm:block">
              {"Gateway Monitor"}
            </p>
          </div>
        </Link>

        {/* right: actions */}
        <div className="flex items-center gap-3">
          {/* last collected + refresh */}
          <div className="hidden items-center gap-2 sm:flex">
            <span className="text-xs text-muted-foreground">
              {"上次采集 "}
              <span className="font-medium text-foreground">{relativeTime(lastCollectedAt)}</span>
            </span>
            <Tooltip delayDuration={200}>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleRefresh}
                  disabled={syncing}
                  className="gap-1.5 border-border bg-background text-foreground hover:bg-muted"
                  aria-label="刷新视图"
                >
                  <RefreshCw className={cn("size-3.5", syncing && "animate-spin")} />
                  {"刷新视图"}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="max-w-xs text-xs">
                <p>{"重新拉取最新的快照数据。"}</p>
                <p className="mt-1 text-muted-foreground">
                  {"提示：实际采集由后台定时任务执行，如需立即采集请到具体渠道点 \"同步\"。"}
                </p>
              </TooltipContent>
            </Tooltip>
          </div>

          {/* mobile-only refresh (no tooltip / no timestamp to save space) */}
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={syncing}
            className="gap-1.5 border-border bg-background text-foreground hover:bg-muted sm:hidden"
            aria-label="刷新视图"
          >
            <RefreshCw className={cn("size-3.5", syncing && "animate-spin")} />
            {"刷新"}
          </Button>

          {hasUpdate ? (
            <Tooltip delayDuration={200}>
              <TooltipTrigger asChild>
                <Button
                  asChild
                  variant="outline"
                  size="sm"
                  className="h-8 gap-1.5 border-warning/30 bg-warning/10 px-2 text-xs font-semibold text-warning shadow-[0_0_0_1px_rgba(245,158,11,0.10)] hover:bg-warning/15 hover:text-warning"
                  aria-label="检测到新版本，前往运维中心更新"
                >
                  <Link to="/settings">
                    <Sparkles className="size-3.5" />
                    <span>可更新</span>
                    {versionInfo?.latest_short ? (
                      <span className="hidden font-mono text-[10px] sm:inline">{versionInfo.latest_short}</span>
                    ) : null}
                  </Link>
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="text-xs">
                {versionInfo
                  ? "当前 " + versionInfo.current.short_commit + " · 最新 " + (versionInfo.latest_short ?? "未知") + "，点击前往运维中心更新"
                  : "检测到新版本，点击前往运维中心更新"}
              </TooltipContent>
            </Tooltip>
          ) : null}

          {/* GitHub repo link */}
          <Tooltip delayDuration={200}>
            <TooltipTrigger asChild>
              <Button
                asChild
                variant="outline"
                size="icon"
                className="size-8 border-border bg-background text-foreground hover:bg-muted"
                aria-label="GitHub 仓库"
              >
                <a
                  href={repoURL}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Github className="size-3.5" />
                </a>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="text-xs">
              {`GitHub · ${repoName}`}
            </TooltipContent>
          </Tooltip>

          <Tooltip delayDuration={200}>
            <TooltipTrigger asChild>
              <Button
                asChild
                variant="outline"
                size="icon"
                className="size-8 border-border bg-background text-foreground hover:bg-muted"
                aria-label="运维中心"
              >
                <Link to="/settings">
                  <Settings className="size-3.5" />
                </Link>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="text-xs">
              {"运维中心"}
            </TooltipContent>
          </Tooltip>

          {/* theme toggle */}
          <Tooltip delayDuration={200}>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                className="size-8 border-border bg-background text-foreground hover:bg-muted"
                aria-label="切换主题"
              >
                {mounted && theme === "dark" ? (
                  <Moon className="size-3.5" />
                ) : (
                  <Sun className="size-3.5" />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom" className="text-xs">
              {mounted && theme === "dark" ? "深色模式 · 点击切换浅色" : "浅色模式 · 点击切换深色"}
            </TooltipContent>
          </Tooltip>

          {/* logout — 鉴权关闭时整个按钮不显示 */}
          {authDisabled ? null : (
            <Tooltip delayDuration={200}>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={logout}
                  className="size-8 border-border bg-background text-foreground hover:bg-muted"
                  aria-label="退出登录"
                >
                  <LogOut className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="text-xs">
                {username ? `${username} · 退出登录` : "退出登录"}
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      </div>
    </header>
  )
}
